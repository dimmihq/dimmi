/*
|--------------------------------------------------------------------------
| Send Email Message
|--------------------------------------------------------------------------
|
| A reply leaves as email from the conversation's address, through the
| install's customer-stream mailer, threaded so it lands in the same
| thread of the customer's mailbox and their answer comes back to the
| same conversation.
|
| Sending is at least once, never twice on purpose: the email is
| recorded with its Message-ID before the provider is called, a retry
| reuses that Message-ID, and a reply already sent is reported again
| without calling the provider. Failures are sorted for the inbox: a
| rate limit waits without spending an attempt, a passing failure is
| retried, and a refusal from the provider fails the reply at once.
|
| Suppressed addresses are never mailed. A suppressed customer fails
| the reply; a suppressed copy is quietly left off.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"time"

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
SendEmailMessage sends one reply by email: the body of the email
channel's Send.

Rules:
  - From is the conversation's address, else the workspace's default.
  - Replies carry In-Reply-To, References, a reply token in Reply-To
    and the loop stamp.
  - Suppressed recipients are never mailed.
  - A reply already sent is not sent again.

Errors: *[contracts.SendError] wrapping the channel sentinels,
*[contracts.RateLimitedError], or a wrapped database error.
*/
type SendEmailMessage struct {
	DB        *database.DB
	Clock     clock.Clock
	Mail      *mail.Manager
	IDs       *ids.Registry
	Config    appconfig.EmailConfig
	Stamps    inbound.Stamps
	Transport string
}

/*
outbound is a reply being sent: the address it leaves from, the
conversation and workspace it belongs to, and the email's row.
*/
type outbound struct {
	msg       contracts.OutboundMessage
	address   models.EmailAddress
	workspace models.Workspace
	row       models.OutboundEmail
	draft     inbound.Draft
}

/*
Send sends the reply and reports what the provider said.
*/
func (a *SendEmailMessage) Send(
	ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage,
) (contracts.SendResult, error) {
	ctx = tenancy.With(ctx, msg.WorkspaceID.String())
	out := &outbound{msg: msg}

	sent, err := a.prepare(ctx, conn, out)
	if err != nil || sent {
		return a.result(out), err
	}

	message := inbound.Compose(out.draft)
	message.Tags = map[string]string{"stream": "customer", "kind": out.row.Kind}

	mailer, err := a.Mail.Mailer(a.Config.Mailer)
	if err != nil {
		return contracts.SendResult{}, &contracts.SendError{Code: "no_sender_available", Message: err.Error()}
	}

	res, err := mailer.Send(ctx, message)
	if err != nil {
		return contracts.SendResult{}, a.failed(ctx, out, err)
	}

	if err := a.record(ctx, out, res); err != nil {
		return contracts.SendResult{}, err
	}

	return a.result(out), nil
}

/*
prepare loads the reply's address and recipients, composes its draft
and records it. It reports true when the reply was sent before.
*/
func (a *SendEmailMessage) prepare(ctx context.Context, conn models.ChannelConnection, out *outbound) (bool, error) {
	sent := false

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		previous, err := q.FindOutboundEmailByIdempotencyKey(ctx, models.FindOutboundEmailByIdempotencyKeyParams{
			WorkspaceID: out.msg.WorkspaceID, IdempotencyKey: out.msg.IdempotencyKey,
		})
		if err == nil && previous.Status != string(enums.OutboundEmailStatusQueued) {
			out.row, sent = previous, true

			return nil
		}

		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find sent email: %w", err)
		}

		if err := a.load(ctx, q, conn, out); err != nil {
			return err
		}

		if err := a.compose(ctx, q, out, previous); err != nil {
			return err
		}

		return a.save(ctx, q, out, previous)
	})

	return sent, err
}

/*
load finds the address the reply leaves from and the workspace.
*/
func (a *SendEmailMessage) load(
	ctx context.Context, q *models.Queries, conn models.ChannelConnection, out *outbound,
) error {
	var err error

	out.workspace, err = q.FindWorkspace(ctx, out.msg.WorkspaceID)
	if err != nil {
		return fmt.Errorf("find workspace: %w", err)
	}

	options, err := readOptions(out.msg.Options)
	if err != nil {
		return err
	}

	out.address, err = a.sendingAddress(ctx, q, conn, options.FromAddressID)

	return err
}

/*
sendingAddress returns the address named in the options, else the
connection's address while it is live, else the workspace's default.
*/
func (a *SendEmailMessage) sendingAddress(
	ctx context.Context, q *models.Queries, conn models.ChannelConnection, named string,
) (models.EmailAddress, error) {
	workspaceID := conn.WorkspaceID
	if named != "" {
		id, err := a.IDs.Decode(string(models.PrefixEmailAddress), named)
		if err != nil {
			return models.EmailAddress{}, noSender()
		}

		address, err := q.FindEmailAddressByID(ctx, models.FindEmailAddressByIDParams{WorkspaceID: workspaceID, ID: id})
		if err == nil && address.DeletedAt == nil {
			return address, nil
		}
	}

	address, err := q.FindEmailAddressByConnection(ctx, models.FindEmailAddressByConnectionParams{
		WorkspaceID: workspaceID, ChannelConnectionID: conn.ID,
	})
	if err == nil && address.DeletedAt == nil {
		return address, nil
	}

	address, err = q.FindDefaultEmailAddress(ctx, workspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return address, noSender()
	}

	if err != nil {
		return address, fmt.Errorf("find default email address: %w", err)
	}

	return address, nil
}

/*
save records the email before the provider sees it, or counts another
attempt at one recorded before.
*/
func (a *SendEmailMessage) save(
	ctx context.Context, q *models.Queries, out *outbound, previous models.OutboundEmail,
) error {
	if previous.ID != ids.Nil() {
		out.row = previous

		return q.StartOutboundEmailAttempt(ctx, models.StartOutboundEmailAttemptParams{
			Now: a.Clock.Now(), WorkspaceID: previous.WorkspaceID, ID: previous.ID,
		})
	}

	metadata := jsonOf(map[string]string{"idempotency_key": out.msg.IdempotencyKey, "stream": "customer"})

	var err error

	d := out.draft
	out.row, err = q.CreateOutboundEmail(ctx, models.CreateOutboundEmailParams{
		WorkspaceID: out.msg.WorkspaceID, MessageID: &out.msg.MessageID, ConversationID: &out.msg.ConversationID,
		EmailAddressID: &out.address.ID, Kind: string(outboundKind(out.msg)), FromAddress: d.From.Email,
		ToAddresses: emailsOf(d.To), CcAddresses: emailsOf(d.Cc), BccAddresses: []string{}, Subject: d.Subject,
		MessageIDHeader: d.MessageID, Transport: a.Transport, Metadata: metadata, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record outbound email: %w", err)
	}

	return q.StartOutboundEmailAttempt(ctx, models.StartOutboundEmailAttemptParams{
		Now: a.Clock.Now(), WorkspaceID: out.row.WorkspaceID, ID: out.row.ID,
	})
}

/*
record notes that the provider accepted the email and files its
Message-IDs under the conversation, ours and any the provider gave it.
*/
func (a *SendEmailMessage) record(ctx context.Context, out *outbound, res mail.SendResult) error {
	return within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		var err error

		out.row, err = q.MarkOutboundEmailSent(ctx, models.MarkOutboundEmailSentParams{
			ProviderMessageID: optional(res.ProviderMessageID), Now: a.Clock.Now(),
			WorkspaceID: out.row.WorkspaceID, ID: out.row.ID,
		})
		if err != nil {
			return fmt.Errorf("mark email sent: %w", err)
		}

		refs := map[string]enums.EmailRefKind{out.row.MessageIDHeader: enums.EmailRefKindOutbound}
		if res.MessageID != "" && res.MessageID != out.row.MessageIDHeader {
			refs[res.MessageID] = enums.EmailRefKindOutboundProvider
		}

		for id, kind := range refs {
			err := q.CreateEmailMessageRef(ctx, models.CreateEmailMessageRefParams{
				WorkspaceID: out.row.WorkspaceID, RfcMessageID: id, ConversationID: out.msg.ConversationID,
				MessageID: &out.msg.MessageID, Kind: string(kind), Now: a.Clock.Now(),
			})
			if err != nil {
				return fmt.Errorf("file sent message id: %w", err)
			}
		}

		return q.RecordEmailAddressOutbound(ctx, models.RecordEmailAddressOutboundParams{
			Now: a.Clock.Now(), WorkspaceID: out.address.WorkspaceID, ID: out.address.ID,
		})
	})
}

/*
failed records a failed attempt and returns the failure as the inbox
reads it.
*/
func (a *SendEmailMessage) failed(ctx context.Context, out *outbound, cause error) error {
	mapped := sendError(cause)
	message := mapped.Error()

	_ = within(context.WithoutCancel(ctx), a.DB, func(ctx context.Context, q *models.Queries) error {
		if !permanent(mapped) {
			return q.NoteOutboundEmailError(ctx, models.NoteOutboundEmailErrorParams{
				LastError: &message, Now: a.Clock.Now(), WorkspaceID: out.row.WorkspaceID, ID: out.row.ID,
			})
		}

		return q.MarkOutboundEmailFailed(ctx, models.MarkOutboundEmailFailedParams{
			Status: string(enums.OutboundEmailStatusFailed), LastError: &message, Now: a.Clock.Now(),
			WorkspaceID: out.row.WorkspaceID, ID: out.row.ID,
		})
	})

	return mapped
}

/*
result is what the inbox learns of a sent reply.
*/
func (a *SendEmailMessage) result(out *outbound) contracts.SendResult {
	if out.row.ID == ids.Nil() {
		return contracts.SendResult{}
	}

	result := contracts.SendResult{
		ChannelMessageID: out.row.MessageIDHeader, Status: enums.DeliveryStatusSent, SentAt: sentTime(out.row),
		Metadata: map[string]any{
			"outbound_email_id": models.PublicID(models.PrefixOutboundEmail, out.row.ID),
			"from":              out.row.FromAddress, "to": out.row.ToAddresses, "cc": out.row.CcAddresses,
			"transport": out.row.Transport, "provider_message_id": out.row.ProviderMessageID,
		},
	}

	if out.msg.ExternalThreadKey == "" {
		result.ExternalThreadKey = out.row.MessageIDHeader
	}

	return result
}

/*
sentTime returns when the provider accepted the email.
*/
func sentTime(row models.OutboundEmail) time.Time {
	if row.SentAt == nil {
		return time.Time{}
	}

	return *row.SentAt
}
