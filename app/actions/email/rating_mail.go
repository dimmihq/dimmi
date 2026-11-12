/*
|--------------------------------------------------------------------------
| Rating Mail
|--------------------------------------------------------------------------
|
| The email that asks a customer to rate a conversation leaves the way
| the team's replies do: from the conversation's address, else the
| workspace's default, as that address itself while its domain is
| verified and from its forwarding address otherwise, through the
| customer-stream mailer. It answers the conversation's thread, so it
| lands beside the replies in the customer's mailbox, and an answer to
| it comes back to the same conversation. A workspace with no address,
| with sending paused, or a customer whose address bounced, is not
| mailed at all.
|
| The email is recorded before the provider sees it, under a key per
| ask, so a retry never sends it twice.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
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
ErrRatingNotSent says a rating email was left unsent on purpose: the
workspace has no address to send from, its sending is paused, or the
customer's address is suppressed.
*/
var ErrRatingNotSent = errors.New("email: rating email not sent")

/*
SendRatingEmail sends the email that asks for a conversation's rating.
*/
type SendRatingEmail struct {
	DB        *database.DB
	Clock     clock.Clock
	Mail      *mail.Manager
	Config    appconfig.EmailConfig
	Senders   *Domains
	Stamps    inbound.Stamps
	Transport string
}

/*
RatingEmail is one rating email: the conversation and the channel it
lives on, the key that makes it go once, the customer, and the content.
PublicID names the ask in its Message-ID.
*/
type RatingEmail struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ConnectionID   ids.UUID
	Key            string
	PublicID       string
	To             mail.Address
	Subject        string
	Text           string
	HTML           string
}

/*
RatingSent is where a rating email went: the address it left from and
its Message-ID.
*/
type RatingSent struct {
	From            string
	MessageIDHeader string
}

/*
Send sends the rating email, or reports the one sent before under the
same key. It returns ErrRatingNotSent when the email may not go.
*/
func (a *SendRatingEmail) Send(ctx context.Context, in RatingEmail) (RatingSent, error) {
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
			return fmt.Errorf("find sent rating email: %w", err)
		}

		row, message, err = a.prepare(ctx, q, in, previous)

		return err
	})

	sent := RatingSent{From: row.FromAddress, MessageIDHeader: row.MessageIDHeader}
	if err != nil || done {
		return sent, err
	}

	return sent, a.deliver(ctx, in, row, message)
}

/*
prepare composes the email in the conversation's thread and records it.
*/
func (a *SendRatingEmail) prepare(ctx context.Context, q *models.Queries, in RatingEmail,
	previous models.OutboundEmail,
) (models.OutboundEmail, mail.Message, error) {
	address, sender, err := a.from(ctx, q, in)
	if err != nil {
		return previous, mail.Message{}, err
	}

	suppressed, err := q.ListSuppressedEmails(ctx, models.ListSuppressedEmailsParams{
		WorkspaceID: in.WorkspaceID, Emails: []string{lower(in.To.Email)}, Now: a.Clock.Now(),
	})
	if err != nil {
		return previous, mail.Message{}, fmt.Errorf("list suppressed addresses: %w", err)
	}
	if len(suppressed) > 0 {
		return previous, mail.Message{}, ErrRatingNotSent
	}

	origin := ratingOrigin{address: address, sender: sender, messageID: previous.MessageIDHeader}
	draft, err := a.draft(ctx, q, in, origin)
	if err != nil {
		return previous, mail.Message{}, err
	}

	message := inbound.Compose(draft)
	message.Tags = map[string]string{"stream": "customer", "kind": string(enums.OutboundEmailKindRating)}

	if previous.ID != ids.Nil() {
		return previous, message, q.StartOutboundEmailAttempt(ctx, models.StartOutboundEmailAttemptParams{
			Now: a.Clock.Now(), WorkspaceID: previous.WorkspaceID, ID: previous.ID,
		})
	}

	row, err := q.CreateOutboundEmail(ctx, models.CreateOutboundEmailParams{
		WorkspaceID: in.WorkspaceID, ConversationID: &in.ConversationID, EmailAddressID: &address.ID,
		Kind: string(enums.OutboundEmailKindRating), FromAddress: draft.From.Email,
		ToAddresses: []string{lower(in.To.Email)}, CcAddresses: []string{}, BccAddresses: []string{},
		Subject: in.Subject, MessageIDHeader: draft.MessageID, Transport: a.Transport,
		Metadata: jsonOf(map[string]string{
			"idempotency_key": in.Key, "stream": "customer", "purpose": "conversation_rating",
		}),
		Now: a.Clock.Now(),
	})
	if err != nil {
		return row, message, fmt.Errorf("record rating email: %w", err)
	}

	return row, message, q.StartOutboundEmailAttempt(ctx, models.StartOutboundEmailAttemptParams{
		Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
}

/*
from returns the address the email leaves from, the one the replies
use, and how it sends.
*/
func (a *SendRatingEmail) from(ctx context.Context, q *models.Queries, in RatingEmail,
) (models.EmailAddress, Sender, error) {
	address, err := q.FindEmailAddressByConnection(ctx, models.FindEmailAddressByConnectionParams{
		WorkspaceID: in.WorkspaceID, ChannelConnectionID: in.ConnectionID,
	})
	if err != nil || address.DeletedAt != nil {
		address, err = q.FindDefaultEmailAddress(ctx, in.WorkspaceID)
	}
	if errors.Is(err, database.ErrNotFound) {
		return address, Sender{}, ErrRatingNotSent
	}
	if err != nil {
		return address, Sender{}, fmt.Errorf("find rating email address: %w", err)
	}

	sender := Sender{Email: address.ForwardingAddress, State: SendingDefaultSender}
	if a.Senders != nil {
		if sender, err = a.Senders.ResolveSender(ctx, q, in.WorkspaceID, address); err != nil {
			return address, sender, err
		}
	}
	if sender.BlockedReason == BlockedSendingPaused {
		return address, sender, ErrRatingNotSent
	}

	return address, sender, nil
}

/*
ratingOrigin is where a rating email goes out from: the address it is
threaded on, the sender that signs it, and the Message-ID an earlier
try used, empty on the first.
*/
type ratingOrigin struct {
	address   models.EmailAddress
	sender    Sender
	messageID string
}

/*
draft threads the email on the conversation's mail and signs it as the
team.
*/
func (a *SendRatingEmail) draft(
	ctx context.Context, q *models.Queries, in RatingEmail, o ratingOrigin,
) (inbound.Draft, error) {
	address, sender, messageID := o.address, o.sender, o.messageID

	thread, err := q.EnsureEmailThread(ctx, models.EnsureEmailThreadParams{
		WorkspaceID: in.WorkspaceID, ConversationID: in.ConversationID, EmailAddressID: &address.ID,
		ReplyToken: inbound.NewReplyToken(), Now: a.Clock.Now(),
	})
	if err != nil {
		return inbound.Draft{}, fmt.Errorf("ensure email thread: %w", err)
	}

	rows, err := q.ListConversationRefs(ctx, models.ListConversationRefsParams{
		WorkspaceID: in.WorkspaceID, ConversationID: in.ConversationID,
	})
	if err != nil {
		return inbound.Draft{}, fmt.Errorf("list conversation refs: %w", err)
	}

	var refs []string
	inReplyTo := ""
	for _, row := range rows {
		refs = append(refs, row.RfcMessageID)
		inReplyTo = row.RfcMessageID
	}

	from := mail.Address{Name: inbound.DisplayName(address.DisplayName), Email: sender.Email}
	if messageID == "" {
		messageID = inbound.MessageID(in.PublicID, domainOf(from.Email))
	}

	d := inbound.Draft{
		From: from, To: []mail.Address{in.To}, Subject: in.Subject, Text: in.Text, HTML: in.HTML,
		MessageID: messageID, InReplyTo: inReplyTo,
		References: inbound.TrimReferences(refs, a.Config.Outbound.ReferencesMax), Automated: true,
		LoopStamp: a.Stamps.Loop(in.WorkspaceID.String(), messageID), Signing: sender.Signing,
	}
	if address.ReplyToMode == string(enums.ReplyToModeToken) {
		d.ReplyTo = mail.Address{
			Name: from.Name, Email: inbound.ReplyAddress(address.ForwardingAddress, thread.ReplyToken),
		}
	}

	return d, nil
}

/*
deliver hands the email to the customer-stream mailer, records the
answer and files its Message-ID under the conversation, so an answer to
it threads back.
*/
func (a *SendRatingEmail) deliver(ctx context.Context, in RatingEmail, row models.OutboundEmail,
	message mail.Message,
) error {
	mailer, err := a.Mail.Mailer(a.Config.Mailer)
	if err != nil {
		return &contracts.SendError{Code: "no_sender_available", Message: err.Error(), Err: contracts.ErrPermanent}
	}

	res, err := mailer.Send(ctx, message)
	if err != nil {
		return a.failed(ctx, row, err)
	}

	return within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		if _, err := q.MarkOutboundEmailSent(ctx, models.MarkOutboundEmailSentParams{
			ProviderMessageID: optional(res.ProviderMessageID), Now: a.Clock.Now(),
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		}); err != nil {
			return fmt.Errorf("mark rating email sent: %w", err)
		}

		return q.CreateEmailMessageRef(ctx, models.CreateEmailMessageRefParams{
			WorkspaceID: row.WorkspaceID, RfcMessageID: row.MessageIDHeader, ConversationID: in.ConversationID,
			Kind: string(enums.EmailRefKindOutbound), Now: a.Clock.Now(),
		})
	})
}

/*
failed records a failed attempt and returns the failure.
*/
func (a *SendRatingEmail) failed(ctx context.Context, row models.OutboundEmail, cause error) error {
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
