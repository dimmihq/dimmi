/*
|--------------------------------------------------------------------------
| Send Channel Message
|--------------------------------------------------------------------------
|
| A reply on a channel that pushes, such as email, leaves after the
| request that wrote it has committed, through its channel's driver. The
| message waits as pending until the provider takes it; then it is sent.
| A provider that refuses for good, such as for an address that cannot
| be reached, fails the message at once with the reason; any other
| failure is tried again, and the message fails when the last attempt
| does. A message no longer pending, already sent or removed, is left
| alone, so a job that runs twice sends once.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/channels"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on sending: how often a message is tried and how long one try
may take.
*/
const (
	sendAttempts = 5
	sendTimeout  = 30 * time.Second
)

/*
The delivery statuses the job moves a message between.
*/
const (
	deliveryPending = "pending"
	deliverySent    = "sent"
	deliveryFailed  = "failed"
)

/*
SendChannelMessageArgs names the message to send.
*/
type SendChannelMessageArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
	MessageID      ids.UUID `json:"message_id"`
}

/*
Kind names the job.
*/
func (SendChannelMessageArgs) Kind() string {
	return "send_channel_message"
}

/*
Options try the job a few times before the message fails.
*/
func (SendChannelMessageArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: sendAttempts}
}

/*
SendChannelMessage sends a pending reply through its channel.
*/
type SendChannelMessage struct {
	DB       *database.DB
	Clock    clock.Clock
	Channels *channels.Manager
}

/*
outbound is a message ready to send: the connection it goes out on, its
driver and what the driver is handed.
*/
type outbound struct {
	conn    models.ChannelConnection
	driver  contracts.Channel
	message contracts.OutboundMessage
}

/*
Handle sends the message and records how it went.
*/
func (w *SendChannelMessage) Handle(ctx context.Context, job *queue.Job[SendChannelMessageArgs]) error {
	args := job.Args
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	out, ok, err := w.prepare(ctx, args)
	if err != nil || !ok {
		return err
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	result, err := out.driver.Send(sendCtx, out.conn, out.message)
	if err == nil {
		return w.record(ctx, args, delivery{status: deliverySent, result: result})
	}

	final := job.Attempt >= job.MaxAttempts || permanent(err)

	status := deliveryPending
	if final {
		status = deliveryFailed
	}

	if recordErr := w.record(ctx, args, delivery{status: status, err: err}); recordErr != nil {
		return recordErr
	}

	if final {
		return queue.Cancel(err)
	}

	return fmt.Errorf("send message: %w", err)
}

/*
permanent reports whether the channel refused for good.
*/
func permanent(err error) bool {
	return errors.Is(err, contracts.ErrPermanent) || errors.Is(err, contracts.ErrRecipientUnreachable) ||
		errors.Is(err, contracts.ErrConnectionBroken)
}

/*
prepare reads the message, its conversation, customer and connection,
and reports false when the message is no longer waiting to be sent.
*/
func (w *SendChannelMessage) prepare(ctx context.Context, args SendChannelMessageArgs) (outbound, bool, error) {
	var (
		out outbound
		ok  bool
	)

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		msg, err := q.FindMessage(ctx, models.FindMessageParams{
			WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID, ID: args.MessageID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find message: %w", err)
		}

		if msg.DeletedAt != nil || msg.DeliveryStatus == nil || *msg.DeliveryStatus != deliveryPending {
			return nil
		}

		out, err = w.outbound(ctx, q, msg)
		ok = err == nil

		return err
	})

	return out, ok, err
}

/*
outbound puts together what the driver is handed for the message.
*/
func (w *SendChannelMessage) outbound(ctx context.Context, q *models.Queries, msg models.Message) (outbound, error) {
	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: msg.WorkspaceID, ID: msg.ConversationID,
	})
	if err != nil {
		return outbound{}, fmt.Errorf("find conversation: %w", err)
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: c.WorkspaceID, ID: c.ChannelConnectionID,
	})
	if err != nil {
		return outbound{}, fmt.Errorf("find channel connection: %w", err)
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return outbound{}, fmt.Errorf("find contact: %w", err)
	}

	driver, err := w.Channels.Driver(conn.Type)
	if err != nil {
		return outbound{}, queue.Cancel(err)
	}

	return outbound{conn: conn, driver: driver, message: contracts.OutboundMessage{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ConversationNumber: c.Number, MessageID: msg.ID,
		IdempotencyKey: models.PublicID(models.PrefixMessage, msg.ID), ExternalThreadKey: deref(c.ExternalThreadKey),
		Subject: deref(c.Subject), BodyMarkdown: deref(msg.Body),
		Recipient: contracts.Recipient{ContactID: contact.ID, Name: deref(contact.Name), Email: deref(contact.Email)},
		Author:    contracts.OutboundAuthor{Type: msg.AuthorType, ID: orNil(msg.AuthorID)},
	}}, nil
}

/*
delivery is how one try went: the status the message moves to, and
what the channel answered.
*/
type delivery struct {
	status string
	result contracts.SendResult
	err    error
}

/*
record writes how the try went onto the message.
*/
func (w *SendChannelMessage) record(ctx context.Context, args SendChannelMessageArgs, d delivery) error {
	now := w.Clock.Now()
	params := models.UpdateMessageDeliveryParams{
		DeliveryStatus: &d.status, Now: now, WorkspaceID: args.WorkspaceID, ID: args.MessageID,
	}

	if d.err != nil {
		params.DeliveryError = new(d.err.Error())
	}

	if d.status == deliverySent {
		sentAt := d.result.SentAt
		if sentAt.IsZero() {
			sentAt = now
		}

		params.SentAt = &sentAt
		if d.result.ChannelMessageID != "" {
			params.ChannelMessageID = &d.result.ChannelMessageID
		}
	}

	return w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, err := models.New(tx).UpdateMessageDelivery(ctx, params); err != nil {
			return fmt.Errorf("record delivery: %w", err)
		}

		return nil
	})
}

/*
deref returns the text, or empty for none.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

/*
orNil returns the id, or ids.Nil() for none.
*/
func orNil(id *ids.UUID) ids.UUID {
	if id == nil {
		return ids.Nil()
	}

	return *id
}
