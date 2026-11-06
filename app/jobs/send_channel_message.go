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
| does. A provider asking to slow down postpones the try without using
| up an attempt. A message no longer pending, already sent or removed,
| is left alone, so a job that runs twice sends once. Open screens learn
| when a reply is sent or fails.
|
*/

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/channels"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
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
	Events   *events.Dispatcher
	Channels *channels.Manager
	Disks    FileDisks
}

/*
FileDisks finds the disk a stored file lives on by its name.
*/
type FileDisks interface {
	/*
		Disk returns the disk with the name.
	*/
	Disk(name string) (filesystem.Filesystem, error)
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
		return w.record(ctx, args, delivery{status: enums.DeliveryStatusSent, result: result, conn: out.conn})
	}

	if limited, ok := errors.AsType[*contracts.RateLimitedError](err); ok {
		return queue.Postpone(limited.RetryAfter)
	}

	final := job.Attempt >= job.MaxAttempts || permanent(err)

	status := enums.DeliveryStatusPending
	if final {
		status = enums.DeliveryStatusFailed
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
	for _, sentinel := range []error{
		contracts.ErrPermanent, contracts.ErrRecipientUnreachable, contracts.ErrConnectionBroken,
		contracts.ErrWindowClosed, contracts.ErrContentRejected, contracts.ErrUnsupported,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}

	return false
}

/*
prepare reads the message, its conversation, customer and connection,
and reports false when the message is no longer waiting to be sent. A
reply held for undo waits until its window ends, then is released under
its lock, so from here on it can no longer be undone.
*/
func (w *SendChannelMessage) prepare(ctx context.Context, args SendChannelMessageArgs) (outbound, bool, error) {
	var (
		out outbound
		ok  bool
	)

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		msg, err := q.LockMessage(ctx, models.LockMessageParams{
			WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID, ID: args.MessageID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find message: %w", err)
		}

		pending := msg.DeliveryStatus != nil && *msg.DeliveryStatus == string(enums.DeliveryStatusPending)
		if msg.DeletedAt != nil || msg.UndoneAt != nil || !pending {
			return nil
		}

		if err := w.release(ctx, q, msg); err != nil {
			return err
		}

		out, err = w.outbound(ctx, q, msg)
		ok = err == nil

		return err
	})

	return out, ok, err
}

/*
release ends the wait of a reply held for undo once its window is over,
and postpones the try until then otherwise.
*/
func (w *SendChannelMessage) release(ctx context.Context, q *models.Queries, msg models.Message) error {
	if msg.ScheduledSendAt == nil {
		return nil
	}

	if wait := msg.ScheduledSendAt.Sub(w.Clock.Now()); wait > 0 {
		return queue.Postpone(wait)
	}

	err := q.ReleaseScheduledMessage(ctx, models.ReleaseScheduledMessageParams{
		Now: w.Clock.Now(), WorkspaceID: msg.WorkspaceID, ID: msg.ID,
	})
	if err != nil {
		return fmt.Errorf("release reply: %w", err)
	}

	return nil
}

/*
delivery is how one try went: the status the message moves to, what
the channel answered, and the connection it went out on.
*/
type delivery struct {
	status enums.DeliveryStatus
	result contracts.SendResult
	err    error
	conn   models.ChannelConnection
}

/*
record writes how the try went onto the message.
*/
func (w *SendChannelMessage) record(ctx context.Context, args SendChannelMessageArgs, d delivery) error {
	now := w.Clock.Now()
	params := models.UpdateMessageDeliveryParams{
		DeliveryStatus: new(string(d.status)), Now: now, WorkspaceID: args.WorkspaceID, ID: args.MessageID,
	}

	if d.err != nil {
		params.DeliveryError = new(reason(d.err))
	}

	if d.status == enums.DeliveryStatusSent {
		sentAt := d.result.SentAt
		if sentAt.IsZero() {
			sentAt = now
		}

		params.SentAt = &sentAt
		if d.result.ChannelMessageID != "" {
			params.ChannelMessageID = &d.result.ChannelMessageID
		}
	}

	if d.result.Metadata != nil {
		metadata, err := json.Marshal(map[string]any{d.conn.Type: d.result.Metadata})
		if err != nil {
			return fmt.Errorf("encode delivery metadata: %w", err)
		}

		params.Metadata = metadata
	}

	return w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := q.UpdateMessageDelivery(ctx, params); err != nil {
			return fmt.Errorf("record delivery: %w", err)
		}

		if err := w.announce(ctx, tx, args, d.status); err != nil {
			return err
		}

		if d.result.ExternalThreadKey == "" {
			return nil
		}

		return q.SetConversationThreadKey(ctx, models.SetConversationThreadKeyParams{
			ExternalThreadKey: &d.result.ExternalThreadKey, Now: now, WorkspaceID: args.WorkspaceID,
			ID: args.ConversationID,
		})
	})
}

/*
reason says why a try failed, in the channel's own words for people
when it gave them.
*/
func reason(err error) string {
	if refused, ok := errors.AsType[*contracts.SendError](err); ok && refused.Message != "" {
		return refused.Message
	}

	return err.Error()
}

/*
announce tells open screens a reply was sent or failed. A try that will
be made again changes nothing they show.
*/
func (w *SendChannelMessage) announce(
	ctx context.Context, tx *database.Tx, args SendChannelMessageArgs, status enums.DeliveryStatus,
) error {
	if status == enums.DeliveryStatusPending {
		return nil
	}

	return w.Events.Dispatch(ctx, tx, appevents.MessageDeliveryUpdated{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: args.WorkspaceID, OccurredAt: w.Clock.Now(),
			Actor: appevents.Actor{Type: string(enums.AuthorTypeSystem), ID: ids.Nil()},
		},
		MessageRef: appevents.MessageRef{
			ConversationRef: appevents.ConversationRef{ConversationID: args.ConversationID}, MessageID: args.MessageID,
		},
		Status: string(status),
	})
}
