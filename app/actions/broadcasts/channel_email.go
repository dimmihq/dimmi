/*
|--------------------------------------------------------------------------
| Email Broadcasts
|--------------------------------------------------------------------------
|
| Email goes out from the connected address the broadcast chose, as the
| teammate who sends it. Each recipient gets their own email, with their
| own reply token, unsubscribe link and counted links, so a reply opens a
| conversation with that person and nobody else.
|
*/

package broadcasts

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailSender sends broadcasts by email. Mailer sends each email, Links
writes the unsubscribe links and Pacer holds email back while the
address warms up or the install's quota is spent.
*/
type EmailSender struct {
	Mailer *email.SendBroadcastEmail
	Links  Links
	Pacer  *Pacer
}

/*
Ready refuses a broadcast without a connected address on a verified
domain.
*/
func (s *EmailSender) Ready(ctx context.Context, q *models.Queries, row models.Broadcast) error {
	return s.Mailer.Check(ctx, q, row.WorkspaceID, row.EmailAddressID)
}

/*
Deliver sends the recipient's email.
*/
func (s *EmailSender) Deliver(ctx context.Context, d Delivery) (Delivered, error) {
	r := d.Recipient
	if d.Broadcast.EmailAddressID == nil {
		return Delivered{}, &UndeliverableError{Status: "failed", Reason: "no_sending_address"}
	}

	if s.Pacer != nil {
		if err := s.Pacer.Email(ctx, r.WorkspaceID, *d.Broadcast.EmailAddressID); err != nil {
			return Delivered{}, err
		}
	}

	name := ""
	if d.Contact != nil && d.Contact.Name != nil {
		name = *d.Contact.Name
	}

	sent, err := s.Mailer.Send(ctx, email.BroadcastEmail{
		WorkspaceID: r.WorkspaceID, AddressID: *d.Broadcast.EmailAddressID, Key: "broadcast:" + r.ID.String(),
		SenderName: d.Message.SenderName, To: mail.Address{Name: name, Email: r.Address}, Subject: d.Message.Subject,
		Text: d.Message.Text, HTML: d.Message.HTML, ReplyToken: r.ReplyToken, OneClick: s.Links.OneClick(r),
		PublicID: models.PublicID(models.PrefixBroadcastRecipient, r.ID),
	})
	if err != nil {
		return Delivered{}, emailFailure(err)
	}

	return Delivered{MessageIDHeader: &sent.MessageIDHeader, OutboundEmailID: &sent.OutboundEmailID}, nil
}

/*
emailFailure sorts a failed send: an address or a refusal that will not
change fails the recipient, anything else is tried again.
*/
func emailFailure(err error) error {
	if errors.Is(err, email.ErrBroadcastUnverified) || errors.Is(err, email.ErrBroadcastAddress) ||
		errors.Is(err, email.ErrBroadcastPaused) {
		return &UndeliverableError{Status: "failed", Reason: "sending_address_unavailable"}
	}

	if sendErr, ok := errors.AsType[*contracts.SendError](err); ok && email.PermanentFailure(err) {
		return &UndeliverableError{Status: "failed", Reason: sendErr.Code}
	}

	return err
}
