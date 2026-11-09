/*
|--------------------------------------------------------------------------
| Moving Requests Along
|--------------------------------------------------------------------------
|
| The team moves a request through its statuses, approves or rejects
| what waits for approval, and merges duplicates. Moving a request can
| tell everyone behind it, once each, with a note and the changelog
| update that shipped it. Merging carries votes and comments to the
| request kept, counting each customer once, and the merged request's
| page sends readers on to it.
|
*/

package feedback

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	fmail "github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/mail/feedback"
)

/*
Failures of moving requests.
*/
var (
	ErrMergeSelf = fault.New(fault.Invalid, "merge_into_self",
		"Pick another request to merge this one into.").OnField("target_id")
	ErrModerationInvalid = fault.New(fault.Invalid, "moderation_invalid",
		"The moderation is approved or spam.").OnField("moderation")
)

/*
StatusInput moves a request: Notify tells its voters and author, with
Message as the team's note, and Entry is the changelog update that
shipped it.
*/
type StatusInput struct {
	Membership authz.Membership
	Post       ids.UUID
	Status     string
	Entry      *ids.UUID
	Notify     bool
	Message    string
}

/*
SetStatus moves a request and tells the people behind it when asked.
*/
func (p *Posts) SetStatus(ctx context.Context, in StatusInput) (Post, error) {
	if !in.Membership.Can("feedback:write") {
		return Post{}, exceptions.ErrForbidden
	}
	status, ok := StatusOf(in.Status)
	if !ok {
		return Post{}, ErrStatusInvalid
	}
	ws := in.Membership.Workspace.ID
	var out Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockPost(ctx, q, ws, in.Post)
		if err != nil {
			return err
		}
		row, err := q.SetFeedbackStatus(ctx, models.SetFeedbackStatusParams{
			Status: status.Key, ChangelogEntryID: in.Entry, Now: new(p.Clock.Now()), WorkspaceID: ws, ID: current.ID,
		})
		if err != nil {
			return fmt.Errorf("move feature request: %w", err)
		}
		if in.Notify && current.Status != status.Key {
			if err := p.tell(ctx, tx, news{post: row, status: status, message: in.Message}); err != nil {
				return err
			}
		}
		out, err = one(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
news is a request that moved, its new status and the team's note.
*/
type news struct {
	post    models.FeedbackPost
	status  Status
	message string
}

/*
tell queues one mail to each voter and the author of the request.
*/
func (p *Posts) tell(ctx context.Context, tx *database.Tx, n news) error {
	q, post, s, message := models.New(tx), n.post, n.status, n.message
	h, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: post.WorkspaceID, ID: post.HelpCenterID})
	if err != nil {
		return fmt.Errorf("find feature request help center: %w", err)
	}
	people, err := q.ListFeedbackRecipients(ctx, models.ListFeedbackRecipientsParams{
		WorkspaceID: post.WorkspaceID, PostID: post.ID,
	})
	if err != nil {
		return fmt.Errorf("list feature request recipients: %w", err)
	}
	root := p.Addresses.Root(h)
	moved := feedback.Moved{
		HelpCenter: h.Name, Brand: brandOf(h), Title: post.Title, Status: s.Name,
		Message: strings.TrimSpace(message), URL: root + PostPath(post),
	}
	if post.ChangelogEntryID != nil {
		entry, err := q.FindChangelogEntry(ctx, models.FindChangelogEntryParams{
			WorkspaceID: post.WorkspaceID, ID: *post.ChangelogEntryID,
		})
		if err == nil && entry.Status == "published" {
			moved.Update = root + "/changelog/" + entry.Slug
		}
	}
	for _, person := range people {
		if person.Email == nil {
			continue
		}
		to := fmail.Address{Email: *person.Email, Name: deref(person.Name)}
		if err := p.Mail.To(to).QueueTx(ctx, tx, appmail.FeedbackStatusMail{Moved: moved}); err != nil {
			return fmt.Errorf("queue feature request mail: %w", err)
		}
	}
	return nil
}

/*
PostPath is where a request's page is under its portal.
*/
func PostPath(post models.FeedbackPost) string {
	return "/feedback/" + strconv.Itoa(int(post.Number)) + "-" + post.Slug
}

/*
Moderate approves a request waiting for approval, or marks it spam.
*/
func (p *Posts) Moderate(ctx context.Context, m authz.Membership, id ids.UUID, moderation string) (Post, error) {
	if !m.Can("feedback:write") {
		return Post{}, exceptions.ErrForbidden
	}
	if moderation != "approved" && moderation != "spam" {
		return Post{}, ErrModerationInvalid
	}
	ws := m.Workspace.ID
	var out Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := lockPost(ctx, q, ws, id); err != nil {
			return err
		}
		row, err := q.SetFeedbackModeration(ctx, models.SetFeedbackModerationParams{
			Moderation: moderation, Now: p.Clock.Now(), WorkspaceID: ws, ID: id,
		})
		if err != nil {
			return fmt.Errorf("moderate feature request: %w", err)
		}
		out, err = one(ctx, q, ws, row)
		return err
	})
	return out, err
}

/*
Merge folds a duplicate into the request kept: its votes, once per
customer, and its comments move over, and it points readers on.
*/
func (p *Posts) Merge(ctx context.Context, m authz.Membership, source, target ids.UUID) (Post, error) {
	if !m.Can("feedback:write") {
		return Post{}, exceptions.ErrForbidden
	}
	if source == target {
		return Post{}, ErrMergeSelf
	}
	ws := m.Workspace.ID
	var out Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		from, err := lockPost(ctx, q, ws, source)
		if err != nil {
			return err
		}
		into, err := lockPost(ctx, q, ws, target)
		if err != nil {
			return err
		}
		if err := p.carry(ctx, q, from, into); err != nil {
			return err
		}
		out, err = p.reload(ctx, q, ws, into.ID)
		return err
	})
	return out, err
}

/*
carry moves the duplicate's votes and comments to the request kept,
points the duplicate at it and recounts both.
*/
func (p *Posts) carry(ctx context.Context, q *models.Queries, from, into models.FeedbackPost) error {
	move := models.MoveFeedbackVotesParams{WorkspaceID: from.WorkspaceID, SourceID: from.ID, TargetID: into.ID}
	if err := q.MoveFeedbackVotes(ctx, move); err != nil {
		return fmt.Errorf("move feature request votes: %w", err)
	}
	if err := q.MoveFeedbackComments(ctx, models.MoveFeedbackCommentsParams(move)); err != nil {
		return fmt.Errorf("move feature request comments: %w", err)
	}
	if _, err := q.MergeFeedbackPost(ctx, models.MergeFeedbackPostParams{
		TargetID: &into.ID, Now: p.Clock.Now(), WorkspaceID: from.WorkspaceID, ID: from.ID,
	}); err != nil {
		return fmt.Errorf("merge feature request: %w", err)
	}
	if err := recount(ctx, q, from); err != nil {
		return err
	}
	return recount(ctx, q, into)
}

/*
brandOf is the help center's brand color, with the default when it has
none.
*/
func brandOf(h models.HelpCenter) string {
	if c := helpcenters.SettingsOf(h).PrimaryColor; c != "" {
		return c
	}
	return "#10b981"
}
