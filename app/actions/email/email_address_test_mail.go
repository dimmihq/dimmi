/*
|--------------------------------------------------------------------------
| Forwarding Test
|--------------------------------------------------------------------------
|
| Setting up forwarding happens in another mail system, so the only way
| to know it works is to try it: a test email goes to the public
| address, and when the forward is in place it arrives back at the
| forwarding address carrying its test stamp. That arrival marks the
| address verified and never becomes a conversation. The same email
| proves replies can leave from the address too.
|
*/

package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
AddressTest is a sent forwarding test: where it went and the
Message-ID it carries.
*/
type AddressTest struct {
	SentTo    string
	MessageID string
}

/*
SendEmailAddressTest mails a forwarding test to an address's public
address.

Rules:
  - The caller needs email:manage.
  - The test carries the address's test stamp, so its arrival marks
    the address verified.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMailUnavailable] when the mailer refuses it, or a
wrapped database error.
*/
type SendEmailAddressTest struct {
	DB        *database.DB
	Clock     clock.Clock
	Mail      *mail.Manager
	Config    appconfig.EmailConfig
	Stamps    inbound.Stamps
	Transport string
}

/*
Handle sends the test.
*/
func (a *SendEmailAddressTest) Handle(ctx context.Context, m authz.Membership, id ids.UUID) (AddressTest, error) {
	if !m.Can("email:manage") {
		return AddressTest{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	var (
		address models.EmailAddress
		row     models.OutboundEmail
		message mail.Message
	)

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		found, err := findAddress(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		address = found.EmailAddress
		message = a.compose(address)

		row, err = a.save(ctx, q, address, message)

		return err
	})
	if err != nil {
		return AddressTest{}, err
	}

	if err := a.send(ctx, row, message); err != nil {
		return AddressTest{}, err
	}

	return AddressTest{SentTo: address.Address, MessageID: row.MessageIDHeader}, nil
}

/*
compose writes the test email.
*/
func (a *SendEmailAddressTest) compose(address models.EmailAddress) mail.Message {
	domain := domainOf(address.ForwardingAddress)

	return inbound.Compose(inbound.Draft{
		From:      mail.Address{Name: inbound.DisplayName(address.DisplayName), Email: address.ForwardingAddress},
		To:        []mail.Address{{Email: address.Address}},
		Subject:   "Forwarding test for " + address.Address,
		MessageID: "test." + inbound.NewReplyToken() + "@" + domain,
		Text: "This is a test from dimmi. When forwarding from " + address.Address + " is set up, it arrives " +
			"back at " + address.ForwardingAddress + " and the address shows as receiving.",
		HTML: "<p>This is a test from dimmi.</p><p>When forwarding from " + address.Address + " is set up, it " +
			"arrives back at " + address.ForwardingAddress + " and the address shows as receiving.</p>",
		TestStamp: a.Stamps.Test(address.ID.String()),
	})
}

/*
save records the test before it is sent.
*/
func (a *SendEmailAddressTest) save(
	ctx context.Context, q *models.Queries, address models.EmailAddress, message mail.Message,
) (models.OutboundEmail, error) {
	row, err := q.CreateOutboundEmail(ctx, models.CreateOutboundEmailParams{
		WorkspaceID: address.WorkspaceID, EmailAddressID: &address.ID, Kind: string(enums.OutboundEmailKindTest),
		FromAddress: message.From.Email, ToAddresses: emailsOf(message.To), CcAddresses: []string{},
		BccAddresses: []string{}, Subject: message.Subject,
		MessageIDHeader: strings.Trim(message.Headers["Message-ID"], "<>"),
		Transport:       a.Transport, Metadata: []byte(`{"stream":"customer"}`), Now: a.Clock.Now(),
	})
	if err != nil {
		return row, fmt.Errorf("record test email: %w", err)
	}

	return row, nil
}

/*
send hands the test to the customer-stream mailer and records the
outcome.
*/
func (a *SendEmailAddressTest) send(ctx context.Context, row models.OutboundEmail, message mail.Message) error {
	mailer, err := a.Mail.Mailer(a.Config.Mailer)
	if err != nil {
		return exceptions.ErrMailUnavailable
	}

	message.Tags = map[string]string{"stream": "customer", "kind": string(enums.OutboundEmailKindTest)}

	res, err := mailer.Send(ctx, message)
	if err != nil {
		reason := sendError(err).Error()

		_ = within(context.WithoutCancel(ctx), a.DB, func(ctx context.Context, q *models.Queries) error {
			return q.MarkOutboundEmailFailed(ctx, models.MarkOutboundEmailFailedParams{
				Status: string(enums.OutboundEmailStatusFailed), LastError: &reason, Now: a.Clock.Now(),
				WorkspaceID: row.WorkspaceID, ID: row.ID,
			})
		})

		return exceptions.ErrMailUnavailable
	}

	return within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		_, err := q.MarkOutboundEmailSent(ctx, models.MarkOutboundEmailSentParams{
			ProviderMessageID: optional(res.ProviderMessageID), Now: a.Clock.Now(),
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		})

		return err
	})
}
