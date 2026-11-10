/*
|--------------------------------------------------------------------------
| Broadcast Work
|--------------------------------------------------------------------------
|
| The jobs behind a send. Starting freezes the audience once its time
| has come. Dispatching hands out a channel's worth of recipients every
| few seconds and waits, without spending attempts, until nobody is
| left. Each recipient is sent by its own job, and a recipient handed out
| but never sent comes back when its lease runs out.
|
*/

package broadcasts

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
longestWait is the longest a dispatcher sleeps: a recipient handed out
holds the queue until its lease runs out, so a dispatcher that slept
until then would finish a broadcast minutes after its last send.
*/
const longestWait = time.Minute

/*
Start freezes a broadcast's audience once its time has come, then
starts dispatching.
*/
func (b *Broadcasts) Start(ctx context.Context, ws, id ids.UUID) error {
	var row models.Broadcast

	err := b.locked(ctx, ws, id, func(ctx context.Context, tx *database.Tx, found models.Broadcast) error {
		row = found
		if row.Status == StatusScheduled {
			if startAt(row, b.Clock.Now()).After(b.Clock.Now().Add(time.Minute)) {
				return nil
			}

			var err error
			row, err = b.move(ctx, tx, row, move{status: StatusSending})

			return err
		}

		return nil
	})
	if err != nil || row.Status != StatusSending {
		return gone(err)
	}

	if err := b.freeze(ctx, row); err != nil {
		return err
	}

	if err := b.Queue.Push(ctx, jobs.TallyBroadcastArgs{WorkspaceID: ws, BroadcastID: id}); err != nil {
		return err
	}

	return b.Queue.Push(ctx, jobs.DispatchBroadcastArgs{WorkspaceID: ws, BroadcastID: id})
}

/*
Dispatch hands the due recipients to senders and comes back later
while any are left, or finishes the broadcast when none are.
*/
func (b *Broadcasts) Dispatch(ctx context.Context, ws, id ids.UUID) error {
	var (
		leased []models.BroadcastRecipient
		next   *time.Time
		row    models.Broadcast
	)

	err := b.locked(ctx, ws, id, func(ctx context.Context, tx *database.Tx, found models.Broadcast) error {
		row = found
		if row.Status != StatusSending {
			return nil
		}

		q, now := models.New(tx), b.Clock.Now()

		var err error

		leased, err = q.LeaseDueBroadcastRecipients(ctx, models.LeaseDueBroadcastRecipientsParams{
			WorkspaceID: ws, BroadcastID: id, Now: now, RowLimit: b.perRun(row.Channel),
			LeaseUntil: now.Add(time.Duration(max(b.Config.LeaseSeconds, 30)) * time.Second),
		})
		if err != nil {
			return wrap("lease recipients", err)
		}

		for _, r := range leased {
			args := jobs.SendBroadcastRecipientArgs{WorkspaceID: ws, RecipientID: r.ID}
			if err := b.Queue.PushTx(ctx, tx, args); err != nil {
				return err
			}
		}

		next, err = b.nextDue(ctx, q, ws, id)
		if err != nil || next != nil || len(leased) > 0 {
			return err
		}

		row, err = b.move(ctx, tx, row, move{status: StatusSent})

		return err
	})
	if err != nil || row.Status != StatusSending {
		return gone(err)
	}

	if len(leased) > 0 {
		next = nil
	}

	return b.comeBack(ctx, ws, id, next)
}

/*
comeBack tallies the broadcast and waits for the next round.
*/
func (b *Broadcasts) comeBack(ctx context.Context, ws, id ids.UUID, next *time.Time) error {
	if err := b.Queue.Push(ctx, jobs.TallyBroadcastArgs{WorkspaceID: ws, BroadcastID: id}); err != nil {
		return err
	}

	wait := b.dispatchEvery()
	if next != nil && next.Sub(b.Clock.Now()) > wait {
		wait = min(next.Sub(b.Clock.Now()), longestWait)
	}

	return queue.Postpone(wait)
}

/*
nextDue returns when the next queued recipient is due, nil when none is
queued.
*/
func (b *Broadcasts) nextDue(ctx context.Context, q *models.Queries, ws, id ids.UUID) (*time.Time, error) {
	next, err := q.NextBroadcastRecipientDue(ctx, models.NextBroadcastRecipientDueParams{
		WorkspaceID: ws, BroadcastID: id,
	})
	if errors.Is(err, database.ErrNotFound) || next.IsZero() {
		return nil, nil
	}

	if err != nil {
		return nil, wrap("next due recipient", err)
	}

	return &next, nil
}

/*
perRun is how many recipients one dispatch hands out on the channel.
*/
func (b *Broadcasts) perRun(channel string) int32 {
	perMinute := b.Config.MessengerPerMinute

	switch channel {
	case ChannelEmail:
		perMinute = b.Config.EmailPerMinute
	case ChannelSlack:
		perMinute = b.Config.SlackPerMinute
	}

	perRun := max(perMinute*int(b.dispatchEvery()/time.Second)/60, 1)

	return int32Of(min(perRun, max(b.Config.BatchSize, 1)))
}

/*
dispatchEvery is how often a sending broadcast hands out recipients.
*/
func (b *Broadcasts) dispatchEvery() time.Duration {
	return time.Duration(max(b.Config.DispatchSeconds, 1)) * time.Second
}

/*
gone turns a broadcast that no longer exists into nothing to do.
*/
func gone(err error) error {
	if errors.Is(err, exceptions.ErrNotFound) {
		return nil
	}

	return err
}

/*
notFound turns a missing row into the not-found error, naming the step
otherwise.
*/
func notFound(err error, what string) error {
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	return wrap(what, err)
}

/*
int32Of bounds a count to what a query takes.
*/
func int32Of(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n)
}
