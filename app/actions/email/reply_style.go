/*
|--------------------------------------------------------------------------
| Reply Style
|--------------------------------------------------------------------------
|
| Each address chooses how its replies look: plain, as a person writes,
| or branded with the company's color and a footer. Whoever manages the
| address previews a choice on a sample reply before saving it, so the
| preview and the sent mail always come from the same composer.
|
*/

package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Sample content of a style preview.
*/
const (
	sampleReply = "Hi Alex,\n\nThanks for getting in touch. The refund is on its way and should " +
		"reach your account within five days.\n\nLet us know if anything else comes up."
	sampleQuote = "Hello, I was charged twice for my order. Could you look into it?"
)

/*
ReplyStyleInput changes the look of an address's replies: plain or
branded, the brand color and the footer. A nil field stays as it is.
*/
type ReplyStyleInput struct {
	Template       *string
	BrandColor     *string
	FooterMarkdown *string
}

/*
RenderEmailTemplate shows a sample reply from an address in a style,
before the style is saved.

Rules:
  - The caller needs email:manage.
  - Fields of the style left out keep what the address has saved.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type RenderEmailTemplate struct {
	DB *database.DB
}

/*
Handle returns the sample reply.
*/
func (a *RenderEmailTemplate) Handle(
	ctx context.Context, m authz.Membership, id ids.UUID, in ReplyStyleInput,
) (mail.Message, error) {
	if !m.Can("email:manage") {
		return mail.Message{}, exceptions.ErrForbidden
	}

	var row models.FindEmailAddressRow

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		row, err = findAddress(ctx, models.New(tx), m.Workspace.ID, id)

		return err
	})
	if err != nil {
		return mail.Message{}, err
	}

	settings := updateParams(row.EmailAddress, UpdateEmailAddressInput{Style: in}).Settings

	style, err := replyStyle(settings)
	if err != nil {
		return mail.Message{}, err
	}

	return inbound.Compose(inbound.Draft{
		From: mail.Address{Name: row.EmailAddress.DisplayName, Email: row.EmailAddress.Address},
		Text: sampleReply, HTML: "<p>" + strings.ReplaceAll(sampleReply, "\n\n", "</p><p>") + "</p>",
		Quote: &inbound.Quote{Header: "Alex wrote:", Text: sampleQuote}, Style: style,
	}), nil
}

/*
dress closes the reply with its signature, frames it in its address's
style, and quotes the message it answers.
*/
func (a *SendEmailMessage) dress(ctx context.Context, q *models.Queries, out *outbound) error {
	var err error

	if out.draft.Signature, err = a.signature(ctx, q, out); err != nil {
		return err
	}

	if out.draft.Style, err = replyStyle(out.address.Settings); err != nil {
		return err
	}

	if out.draft.Attachments, err = readFiles(ctx, out.msg.Attachments); err != nil {
		return err
	}

	return a.quote(ctx, q, out)
}

/*
replyStyle returns the style of replies an address's settings choose.
*/
func replyStyle(raw []byte) (inbound.Style, error) {
	settings := models.ReadEmailAddressSettings(raw)

	style, err := inbound.RenderStyle(settings.Template, settings.BrandColor, settings.FooterMarkdown)
	if err != nil {
		return inbound.Style{}, fmt.Errorf("render reply style: %w", err)
	}

	return style, nil
}
