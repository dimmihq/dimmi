/*
|--------------------------------------------------------------------------
| Broadcast Mail
|--------------------------------------------------------------------------
|
| A broadcast email leaves from one of the workspace's own connected
| addresses, and only while that address's domain is verified and
| signed: bulk mail from a shared domain or a forwarding address would
| spend a reputation that is not the workspace's. Answers come back to
| the address's forwarding address with the recipient's reply token, so
| each one opens a conversation. The email is recorded before the
| provider sees it, under a key per recipient, so a retry never sends
| it twice, and bounces and complaints find it by its Message-ID.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
Why an address cannot send broadcasts.
*/
var (
	ErrBroadcastAddress = fault.New(fault.Invalid, "broadcast_address_missing",
		"Choose one of your connected email addresses to send from.")
	ErrBroadcastUnverified = fault.New(fault.Invalid, "broadcast_address_unverified",
		"Verify the domain of this address before sending broadcasts from it, so they're signed as yours.")
	ErrBroadcastPaused = fault.New(fault.Invalid, "broadcast_sending_paused",
		"Email sending is paused for this workspace, so broadcasts can't go out.")
)

/*
SendBroadcastEmail sends broadcast emails from a connected address.
*/
type SendBroadcastEmail struct {
	DB        *database.DB
	Clock     clock.Clock
	Mail      *mail.Manager
	Config    appconfig.EmailConfig
	Senders   *Domains
	Stamps    inbound.Stamps
	Transport string
}

/*
BroadcastEmail is one broadcast email to send. Key makes the send happen
once; ReplyToken routes answers; OneClick is where mail apps post an
unsubscribe.
*/
type BroadcastEmail struct {
	WorkspaceID ids.UUID
	AddressID   ids.UUID
	Key         string
	SenderName  string
	To          mail.Address
	Subject     string
	Text        string
	HTML        string
	ReplyToken  string
	OneClick    string
	PublicID    string
}

/*
BroadcastSent is where a broadcast email went: its Message-ID and its
row.
*/
type BroadcastSent struct {
	MessageIDHeader string
	OutboundEmailID ids.UUID
}

/*
Check returns why the address cannot send broadcasts, nil when it can.
*/
func (a *SendBroadcastEmail) Check(ctx context.Context, q *models.Queries, ws ids.UUID, addressID *ids.UUID) error {
	_, _, err := a.sender(ctx, q, ws, addressID)

	return err
}

/*
sender returns the address and how it sends, refusing an address that
may not send broadcasts.
*/
func (a *SendBroadcastEmail) sender(ctx context.Context, q *models.Queries, ws ids.UUID, addressID *ids.UUID,
) (models.EmailAddress, Sender, error) {
	if addressID == nil {
		return models.EmailAddress{}, Sender{}, ErrBroadcastAddress
	}

	address, err := q.FindEmailAddressByID(ctx, models.FindEmailAddressByIDParams{WorkspaceID: ws, ID: *addressID})
	if errors.Is(err, database.ErrNotFound) || (err == nil && address.DeletedAt != nil) {
		return address, Sender{}, ErrBroadcastAddress
	}

	if err != nil {
		return address, Sender{}, fmt.Errorf("find email address: %w", err)
	}

	sender, err := a.Senders.ResolveSender(ctx, q, ws, address)
	if err != nil {
		return address, sender, err
	}

	switch {
	case sender.BlockedReason != "":
		return address, sender, ErrBroadcastPaused
	case sender.State != SendingOwnDomain:
		return address, sender, ErrBroadcastUnverified
	}

	return address, sender, nil
}

/*
Send sends one broadcast email, or reports the one sent before under
the same key.
*/
func (a *SendBroadcastEmail) Send(ctx context.Context, in BroadcastEmail) (BroadcastSent, error) {
	ctx = tenancy.With(ctx, in.WorkspaceID.String())

	var (
		row     models.OutboundEmail
		message mail.Message
		done    bool
	)

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		previous, err := q.FindOutboundEmailByIdempotencyKey(ctx, models.FindOutboundEmailByIdempotencyKeyParams{
			WorkspaceID: in.WorkspaceID, IdempotencyKey: in.Key,
		})
		if err == nil && !resendable(previous.Status) {
			row, done = previous, true

			return nil
		}

		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find sent broadcast email: %w", err)
		}

		row, message, err = a.prepare(ctx, q, in, previous)

		return err
	})
	if err != nil || done {
		return BroadcastSent{MessageIDHeader: row.MessageIDHeader, OutboundEmailID: row.ID}, err
	}

	return a.deliver(ctx, row, message)
}

/*
prepare composes the email and records it before the provider sees it.
*/
func (a *SendBroadcastEmail) prepare(ctx context.Context, q *models.Queries, in BroadcastEmail,
	previous models.OutboundEmail,
) (models.OutboundEmail, mail.Message, error) {
	address, sender, err := a.sender(ctx, q, in.WorkspaceID, &in.AddressID)
	if err != nil {
		return previous, mail.Message{}, err
	}

	messageID := previous.MessageIDHeader
	if messageID == "" {
		messageID = inbound.MessageID(in.PublicID, domainOf(sender.Email))
	}

	from := mail.Address{Name: in.SenderName, Email: sender.Email}
	message := inbound.Compose(inbound.Draft{
		From: from, To: []mail.Address{in.To}, Subject: in.Subject, Text: in.Text, HTML: in.HTML,
		ReplyTo: mail.Address{
			Name: in.SenderName, Email: inbound.ReplyAddress(address.ForwardingAddress, in.ReplyToken),
		},
		MessageID: messageID, LoopStamp: a.Stamps.Loop(in.WorkspaceID.String(), messageID), Signing: sender.Signing,
	})
	message.Headers["List-Unsubscribe"] = "<" + in.OneClick + ">"
	message.Headers["List-Unsubscribe-Post"] = "List-Unsubscribe=One-Click"
	message.Headers["Precedence"] = "bulk"
	message.Tags = map[string]string{"stream": "customer", "kind": string(enums.OutboundEmailKindBroadcast)}

	if previous.ID != ids.Nil() {
		return previous, message, q.StartOutboundEmailAttempt(ctx, models.StartOutboundEmailAttemptParams{
			Now: a.Clock.Now(), WorkspaceID: previous.WorkspaceID, ID: previous.ID,
		})
	}

	row, err := q.CreateOutboundEmail(ctx, models.CreateOutboundEmailParams{
		WorkspaceID: in.WorkspaceID, EmailAddressID: &address.ID, Kind: string(enums.OutboundEmailKindBroadcast),
		FromAddress: from.Email, ToAddresses: []string{lower(in.To.Email)}, CcAddresses: []string{},
		BccAddresses: []string{}, Subject: in.Subject, MessageIDHeader: messageID, Transport: a.Transport,
		Metadata: jsonOf(map[string]string{"idempotency_key": in.Key, "stream": "customer"}), Now: a.Clock.Now(),
	})
	if err != nil {
		return row, message, fmt.Errorf("record broadcast email: %w", err)
	}

	return row, message, q.StartOutboundEmailAttempt(ctx, models.StartOutboundEmailAttemptParams{
		Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
}

/*
deliver hands the email to the provider and records the answer.
*/
func (a *SendBroadcastEmail) deliver(ctx context.Context, row models.OutboundEmail, message mail.Message,
) (BroadcastSent, error) {
	sent := BroadcastSent{MessageIDHeader: row.MessageIDHeader, OutboundEmailID: row.ID}

	mailer, err := a.Mail.Mailer(a.Config.Mailer)
	if err != nil {
		return sent, &contracts.SendError{
			Code: "no_sender_available", Message: err.Error(), Err: contracts.ErrPermanent,
		}
	}

	res, err := mailer.Send(ctx, message)
	if err != nil {
		return sent, a.failed(ctx, row, err)
	}

	return sent, within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		_, err := q.MarkOutboundEmailSent(ctx, models.MarkOutboundEmailSentParams{
			ProviderMessageID: optional(res.ProviderMessageID), Now: a.Clock.Now(),
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
		if err != nil {
			return fmt.Errorf("mark broadcast email sent: %w", err)
		}

		return nil
	})
}

/*
failed records a failed attempt and returns the failure as broadcasts
read it.
*/
func (a *SendBroadcastEmail) failed(ctx context.Context, row models.OutboundEmail, cause error) error {
	mapped := sendError(cause)
	message := mapped.Error()

	_ = within(context.WithoutCancel(ctx), a.DB, func(ctx context.Context, q *models.Queries) error {
		if !permanent(mapped) {
			return q.NoteOutboundEmailError(ctx, models.NoteOutboundEmailErrorParams{
				LastError: &message, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
			})
		}

		return q.MarkOutboundEmailFailed(ctx, models.MarkOutboundEmailFailedParams{
			Status: string(enums.OutboundEmailStatusFailed), LastError: &message, Now: a.Clock.Now(),
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
	})

	return mapped
}

/*
PermanentFailure reports whether a broadcast email's failure will never
pass.
*/
func PermanentFailure(err error) bool {
	return permanent(err)
}
