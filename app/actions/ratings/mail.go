/*
|--------------------------------------------------------------------------
| Emailing A Rating Ask
|--------------------------------------------------------------------------
|
| An email conversation's customer is asked by email, from the address
| the conversation's replies use and in the same thread, never from the
| install's own transactional mailer. The email carries five faces, each
| a signed link to the rating page with its score. A workspace with no
| address to send from, with sending paused, or a customer with no
| address or a suppressed one, is skipped and logged: the ask stays
| open and the messenger or the page can still answer it.
|
*/

package ratings

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/log"
	fmail "github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	ratingviews "github.com/dimmihq/dimmi/resources/views/mail/ratings"
)

/*
Mail emails the ask to the conversation's customer.
*/
func (r *Ratings) Mail(ctx context.Context, args jobs.MailConversationRatingArgs) error {
	var (
		in   email.RatingEmail
		skip string
	)

	err := r.inWorkspace(ctx, args.WorkspaceID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		in, skip, err = r.composeMail(ctx, q, args)
		return err
	})
	if err != nil {
		return err
	}

	if skip == "" && r.Email == nil {
		skip = "no customer email sender"
	}
	if skip == "" {
		_, err = r.Email.Send(ctx, in)
		if errors.Is(err, email.ErrRatingNotSent) {
			skip, err = "no address to send from, sending paused or address suppressed", nil
		}
	}
	if skip != "" {
		log.From(ctx).InfoContext(ctx, "rating email skipped", log.String("reason", skip))
	}

	return err
}

/*
composeMail builds the email of the ask, or says why none goes out.
*/
func (r *Ratings) composeMail(ctx context.Context, q *models.Queries, args jobs.MailConversationRatingArgs,
) (email.RatingEmail, string, error) {
	row, err := q.FindConversationRating(ctx, models.FindConversationRatingParams{
		WorkspaceID: args.WorkspaceID, ID: args.RatingID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return email.RatingEmail{}, "ask gone", nil
	}
	if err != nil {
		return email.RatingEmail{}, "", fmt.Errorf("find rating ask: %w", err)
	}

	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: row.WorkspaceID, ID: row.ConversationID,
	})
	if err != nil {
		return email.RatingEmail{}, "", fmt.Errorf("find rated conversation: %w", err)
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: row.WorkspaceID, ID: row.ContactID})
	if err != nil {
		return email.RatingEmail{}, "", fmt.Errorf("find rating contact: %w", err)
	}
	if contact.Email == nil || *contact.Email == "" {
		return email.RatingEmail{}, "customer has no address", nil
	}

	w, err := q.FindWorkspace(ctx, row.WorkspaceID)
	if err != nil {
		return email.RatingEmail{}, "", fmt.Errorf("find rating workspace: %w", err)
	}

	ask := ratingviews.Ask{Workspace: w.Name, Name: deref(contact.Name), Comment: r.Links.Page(row, 0)}
	text := []string{i18n.T(ctx, "mail.conversation_rating.intro", i18n.Args{"workspace": w.Name}), ""}
	faces, lines := r.faceLinks(row)
	ask.Faces = faces
	text = append(text, lines...)

	var html bytes.Buffer
	if err := ratingviews.RatingAsk(ask).Render(ctx, &html); err != nil {
		return email.RatingEmail{}, "", fmt.Errorf("render rating email: %w", err)
	}

	return email.RatingEmail{
		WorkspaceID: row.WorkspaceID, ConversationID: c.ID, ConnectionID: c.ChannelConnectionID,
		Key:      "conversation_rating:" + row.ID.String(),
		PublicID: models.PublicID(models.PrefixConversationRating, row.ID),
		To:       fmail.Address{Email: *contact.Email, Name: deref(contact.Name)},
		Subject:  i18n.T(ctx, "mail.conversation_rating.subject", i18n.Args{"workspace": w.Name}),
		Text:     strings.Join(text, "\n"), HTML: html.String(),
	}, "", nil
}

/*
faceLinks returns the five faces of the ask with their links, and the
same as plain text lines, worst first.
*/
func (r *Ratings) faceLinks(row models.ConversationRating) ([]ratingviews.Face, []string) {
	var faces []ratingviews.Face
	var lines []string

	emojis, labels := Faces(), Labels()
	for score := 1; score <= 5; score++ {
		face := ratingviews.Face{Emoji: emojis[score-1], Label: labels[score-1], URL: r.Links.Page(row, score)}
		faces = append(faces, face)
		lines = append(lines, face.Emoji+" "+face.Label+": "+face.URL)
	}

	return faces, lines
}
