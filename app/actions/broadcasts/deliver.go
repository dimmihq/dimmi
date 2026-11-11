/*
|--------------------------------------------------------------------------
| Reaching One Recipient
|--------------------------------------------------------------------------
|
| Each channel sends in its own way behind one small interface. A
| recipient is sent only while the broadcast is sending and only once:
| the row says it is sent the moment the channel accepts it. A refusal
| that will never change, such as a channel the app was removed from,
| fails the recipient with the reason; a rate limit or a passing
| failure tries again later, a few times at most.
|
*/

package broadcasts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxAttempts is how often a recipient is tried before it fails.
*/
const maxAttempts = 5

/*
Sender reaches recipients on one channel.
*/
type Sender interface {
	/*
		Ready reports why the broadcast cannot go out on the channel, nil
		when it can.
	*/
	Ready(ctx context.Context, q *models.Queries, row models.Broadcast) error

	/*
		Deliver sends the broadcast to one recipient and says where it
		went. An *[UndeliverableError] error fails the recipient for good.
	*/
	Deliver(ctx context.Context, d Delivery) (Delivered, error)
}

/*
Delivery is one recipient to reach: the broadcast, the recipient, the
contact or Slack channel behind it, and the message written for them.
*/
type Delivery struct {
	Broadcast models.Broadcast
	Recipient models.BroadcastRecipient
	Contact   *models.Contact
	Room      *models.ChannelRoom
	Message   Rendered
}

/*
Delivered is where a message went: the email's Message-ID and row, the
Slack post's timestamp, or the Messenger conversation.
*/
type Delivered struct {
	MessageIDHeader *string
	OutboundEmailID *ids.UUID
	ExternalRef     *string
	ConversationID  *ids.UUID
}

/*
UndeliverableError fails a recipient for good: Status is failed, bounced or
suppressed, and Reason a short code the report explains.
*/
type UndeliverableError struct {
	Status string
	Reason string
}

/*
Error says why.
*/
func (e *UndeliverableError) Error() string {
	return "undeliverable: " + e.Reason
}

/*
SendRecipient reaches one recipient.
*/
func (b *Broadcasts) SendRecipient(ctx context.Context, ws, id ids.UUID) error {
	ctx = withTenant(ctx, ws)

	d, ok, err := b.load(ctx, ws, id)
	if err != nil || !ok {
		return gone(err)
	}

	sender, found := b.Senders[d.Broadcast.Channel]
	if !found {
		return b.fail(ctx, d.Recipient, &UndeliverableError{Status: "failed", Reason: "channel_unavailable"})
	}

	sent, err := sender.Deliver(ctx, d)
	if err != nil {
		return b.failed(ctx, d.Recipient, err)
	}

	err = b.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		_, err := q.MarkBroadcastRecipientSent(ctx, models.MarkBroadcastRecipientSentParams{
			WorkspaceID: ws, ID: id, MessageIDHeader: sent.MessageIDHeader, OutboundEmailID: sent.OutboundEmailID,
			ExternalRef: sent.ExternalRef, ConversationID: sent.ConversationID, Now: b.Clock.Now(),
		})

		return ignoreMissing(err, "mark recipient sent")
	})
	if err != nil {
		return err
	}

	return errors.Join(b.recount(ctx, ws, d.Broadcast.ID), b.touched(ctx, d.Recipient))
}

/*
load reads what sending one recipient needs; ok is false when there is
nothing to send, because it went already or the broadcast is not
sending.
*/
func (b *Broadcasts) load(ctx context.Context, ws, id ids.UUID) (Delivery, bool, error) {
	var d Delivery

	ok := false

	err := b.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		if d.Recipient, err = q.FindBroadcastRecipient(ctx, models.FindBroadcastRecipientParams{
			WorkspaceID: ws, ID: id,
		}); err != nil {
			return notFound(err, "find recipient")
		}

		if d.Broadcast, err = find(ctx, q, ws, d.Recipient.BroadcastID); err != nil {
			return err
		}

		if d.Recipient.Status != "queued" || d.Broadcast.Status != StatusSending {
			return nil
		}

		if err := b.subject(ctx, q, &d); err != nil {
			return err
		}

		ok = true
		d.Message, err = b.render(ctx, q, d)

		return err
	})

	return d, ok, err
}

/*
subject loads the contact or Slack channel the recipient stands for.
*/
func (b *Broadcasts) subject(ctx context.Context, q *models.Queries, d *Delivery) error {
	ws := d.Recipient.WorkspaceID
	if d.Recipient.RoomID != nil {
		room, err := q.FindChannelRoom(ctx, models.FindChannelRoomParams{WorkspaceID: ws, ID: *d.Recipient.RoomID})
		if err != nil {
			return notFound(err, "find room")
		}

		d.Room = &room

		return nil
	}

	if d.Recipient.ContactID == nil {
		return nil
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: *d.Recipient.ContactID})
	if err != nil {
		return notFound(err, "find contact")
	}

	d.Contact = &contact

	return nil
}

/*
failed sorts a sending error: a refusal fails the recipient, a rate
limit waits, and anything else is tried again a few times.
*/
func (b *Broadcasts) failed(ctx context.Context, r models.BroadcastRecipient, err error) error {
	if refused, ok := errors.AsType[*UndeliverableError](err); ok {
		return b.fail(ctx, r, refused)
	}

	if paced, ok := errors.AsType[*PacedError](err); ok {
		return b.retryAt(ctx, r, paced.Until, paced.Reason)
	}

	if limited, ok := errors.AsType[*contracts.RateLimitedError](err); ok {
		return b.retryAt(ctx, r, b.Clock.Now().Add(max(limited.RetryAfter, time.Second)), "rate_limited")
	}

	if r.Attempts >= maxAttempts {
		return b.fail(ctx, r, &UndeliverableError{Status: "failed", Reason: "send_failed"})
	}

	return fmt.Errorf("send broadcast recipient: %w", err)
}

/*
fail records that a recipient will not be reached, and why.
*/
func (b *Broadcasts) fail(ctx context.Context, r models.BroadcastRecipient, why *UndeliverableError) error {
	err := b.inWorkspace(ctx, r.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		_, err := q.MarkBroadcastRecipientFailed(ctx, models.MarkBroadcastRecipientFailedParams{
			WorkspaceID: r.WorkspaceID, ID: r.ID, Status: why.Status, Reason: &why.Reason, Now: b.Clock.Now(),
		})

		return ignoreMissing(err, "fail recipient")
	})
	if err != nil {
		return err
	}

	return errors.Join(b.recount(ctx, r.WorkspaceID, r.BroadcastID), b.touched(ctx, r))
}

/*
retryAt puts a recipient back in the queue for a later dispatch.
*/
func (b *Broadcasts) retryAt(ctx context.Context, r models.BroadcastRecipient, at time.Time, why string) error {
	err := b.inWorkspace(ctx, r.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		return wrap("retry recipient", q.RetryBroadcastRecipientAt(ctx, models.RetryBroadcastRecipientAtParams{
			WorkspaceID: r.WorkspaceID, ID: r.ID, DueAt: at, Reason: &why, Now: b.Clock.Now(),
		}))
	})
	if err != nil || r.SeriesStepID == nil {
		return err
	}

	return queue.Postpone(max(at.Sub(b.Clock.Now()), time.Second))
}

/*
recount asks for the broadcast's numbers to be counted again soon.
*/
func (b *Broadcasts) recount(ctx context.Context, ws, id ids.UUID) error {
	return b.Queue.Push(context.WithoutCancel(ctx), jobs.TallyBroadcastArgs{WorkspaceID: ws, BroadcastID: id})
}

/*
ignoreMissing treats a row that changed underneath as nothing to do.
*/
func ignoreMissing(err error, what string) error {
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	return wrap(what, err)
}

/*
sendJob is the job's way into SendRecipient, canceled for good when
the error can never pass.
*/
func sendJob(err error) error {
	if refused, ok := errors.AsType[*UndeliverableError](err); ok {
		return queue.Cancel(refused)
	}

	return err
}
