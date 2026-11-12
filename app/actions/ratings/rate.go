/*
|--------------------------------------------------------------------------
| Answering A Rating
|--------------------------------------------------------------------------
|
| The customer answers from the messenger, as the visitor of the
| conversation, or from the email's page, with the signed link as the
| proof. Either way they pick a score and may add a comment, and can
| change both for a week after the ask went out. Every answer joins the
| conversation's timeline under the customer's name and goes out as a
| ConversationRated event; only the first starts workflows.
|
*/

package ratings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Answer is what the customer sends: a score from 1 to 5, a comment, or
both. A field left nil keeps what was there; an empty comment clears it.
*/
type Answer struct {
	Score   *int
	Comment *string
}

/*
Rating is an ask as the customer sees it: the row, whether it still
takes changes, and the workspace that asks.
*/
type Rating struct {
	Row       models.ConversationRating
	Editable  bool
	Workspace string
}

/*
ForConversation returns the latest ask that went out on the
conversation, nil when none did. The caller has checked the visitor
may see the conversation.
*/
func (r *Ratings) ForConversation(ctx context.Context, workspace, conversation ids.UUID) (*Rating, error) {
	var out *Rating
	err := r.inWorkspace(ctx, workspace, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		row, err := q.FindOpenConversationRating(ctx, models.FindOpenConversationRatingParams{
			WorkspaceID: workspace, ConversationID: conversation,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find rating ask: %w", err)
		}
		out = &Rating{Row: row, Editable: r.editable(row)}
		return nil
	})

	return out, err
}

/*
RateConversation keeps the visitor's answer to the conversation's
latest ask.
*/
func (r *Ratings) RateConversation(ctx context.Context, workspace, conversation ids.UUID, a Answer) (Rating, error) {
	var out Rating
	err := r.inWorkspace(ctx, workspace, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		row, err := q.FindOpenConversationRating(ctx, models.FindOpenConversationRatingParams{
			WorkspaceID: workspace, ConversationID: conversation,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find rating ask: %w", err)
		}
		out, err = r.answer(ctx, txn{tx: tx, q: q}, row, a)
		return err
	})

	return out, err
}

/*
ByToken returns the ask a signed link names, with the workspace's name
for the page.
*/
func (r *Ratings) ByToken(ctx context.Context, token string) (Rating, error) {
	ref, err := r.Links.Read(token)
	if err != nil {
		return Rating{}, exceptions.ErrNotFound
	}

	var out Rating
	err = r.inWorkspace(ctx, ref.WorkspaceID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		row, err := findAsked(ctx, q, ref)
		if err != nil {
			return err
		}
		out = Rating{Row: row, Editable: r.editable(row)}
		out.Workspace, err = workspaceName(ctx, q, ref.WorkspaceID)
		return err
	})

	return out, err
}

/*
RateByToken keeps the answer given on the page a signed link opened.
*/
func (r *Ratings) RateByToken(ctx context.Context, token string, a Answer) (Rating, error) {
	ref, err := r.Links.Read(token)
	if err != nil {
		return Rating{}, exceptions.ErrNotFound
	}

	var out Rating
	err = r.inWorkspace(ctx, ref.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		row, err := findAsked(ctx, q, ref)
		if err != nil {
			return err
		}
		if out, err = r.answer(ctx, txn{tx: tx, q: q}, row, a); err != nil {
			return err
		}
		out.Workspace, err = workspaceName(ctx, q, ref.WorkspaceID)
		return err
	})

	return out, err
}

/*
answer keeps the answer on the ask, records it on the timeline and
dispatches it.
*/
func (r *Ratings) answer(ctx context.Context, t txn, row models.ConversationRating, a Answer) (Rating, error) {
	params, err := r.params(row, a)
	if err != nil {
		return Rating{}, err
	}

	if !r.editable(row) {
		return Rating{}, ErrRatingClosed
	}

	updated, err := t.q.RateConversation(ctx, params)
	if errors.Is(err, database.ErrNotFound) {
		return Rating{}, ErrRatingScore
	}
	if err != nil {
		return Rating{}, fmt.Errorf("rate conversation: %w", err)
	}

	if err := r.record(ctx, t, row, updated); err != nil {
		return Rating{}, err
	}

	return Rating{Row: updated, Editable: true}, nil
}

/*
params checks the answer and turns it into the update.
*/
func (r *Ratings) params(row models.ConversationRating, a Answer) (models.RateConversationParams, error) {
	since := r.Clock.Now().Add(-EditWindow)
	p := models.RateConversationParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, Now: r.Clock.Now(), Since: &since,
	}

	if a.Score != nil {
		if *a.Score < 1 || *a.Score > 5 {
			return p, ErrRatingScore
		}
		score := int16(*a.Score)
		p.Score = &score
	}

	if a.Comment != nil {
		p.SetComment = true
		text := strings.TrimSpace(*a.Comment)
		if text != "" {
			text = truncate(text, MaxComment)
			p.Comment = &text
		}
	}

	if a.Score == nil && a.Comment == nil {
		return p, ErrRatingScore
	}

	return p, nil
}

/*
record writes the answer on the conversation's timeline under the
customer's name and dispatches ConversationRated.
*/
func (r *Ratings) record(ctx context.Context, t txn, before, after models.ConversationRating) error {
	q := t.q
	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: after.WorkspaceID, ID: after.ConversationID,
	})
	if err != nil {
		return fmt.Errorf("find rated conversation: %w", err)
	}

	name := ""
	if contact, err := q.FindContact(ctx, models.FindContactParams{
		WorkspaceID: after.WorkspaceID, ID: after.ContactID,
	}); err == nil {
		name = cmpName(contact)
	}

	w := &conversations.Writer{
		Q: q, Tx: t.tx, Events: r.Events, Now: r.Clock.Now(),
		Actor: conversations.Actor{Type: string(enums.AuthorTypeContact), ID: after.ContactID, Name: name},
	}

	score := int(*after.Score)
	data := map[string]any{"score": score, "label": Labels()[score-1], "channel": after.Channel}
	if after.Comment != nil {
		data["comment"] = *after.Comment
	}

	if err := w.Record(ctx, c, "conversation.rated", data); err != nil {
		return err
	}

	comment := ""
	if after.Comment != nil {
		comment = *after.Comment
	}

	return w.Dispatch(ctx, appevents.ConversationRated{
		Base: w.Base(after.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: c.ID},
		RatingID: after.ID, Score: score, Comment: comment, Channel: after.Channel, MemberID: after.MemberID,
		First: before.Score == nil,
	})
}

/*
editable reports whether the ask went out and still takes changes.
*/
func (r *Ratings) editable(row models.ConversationRating) bool {
	return row.AskedAt != nil && !row.AskedAt.Before(r.Clock.Now().Add(-EditWindow))
}

/*
findAsked returns the ask a link names, once it went out.
*/
func findAsked(ctx context.Context, q *models.Queries, ref Ref) (models.ConversationRating, error) {
	row, err := q.FindConversationRating(ctx, models.FindConversationRatingParams{
		WorkspaceID: ref.WorkspaceID, ID: ref.RatingID,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && row.AskedAt == nil) {
		return row, exceptions.ErrNotFound
	}
	if err != nil {
		return row, fmt.Errorf("find rating ask: %w", err)
	}

	return row, nil
}

func workspaceName(ctx context.Context, q *models.Queries, id ids.UUID) (string, error) {
	w, err := q.FindWorkspace(ctx, id)
	if err != nil {
		return "", fmt.Errorf("find rating workspace: %w", err)
	}
	return w.Name, nil
}

/*
cmpName is how the timeline names the customer: their name, else their
address.
*/
func cmpName(c models.Contact) string {
	if c.Name != nil && *c.Name != "" {
		return *c.Name
	}
	return deref(c.Email)
}

/*
truncate cuts text to at most n characters, on a character boundary.
*/
func truncate(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n])
}
