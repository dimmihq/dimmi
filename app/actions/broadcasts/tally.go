/*
|--------------------------------------------------------------------------
| The Live Tally
|--------------------------------------------------------------------------
|
| A broadcast's numbers are counted from its recipients, never added up
| by hand, so they cannot drift. Counting runs at most every couple of
| seconds while anything happens, and each count reaches open screens
| at once, so a report fills in as the send goes out. An email send that
| bounces or draws complaints past the limits pauses itself on the spot,
| before mailbox providers start distrusting the address.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Why a broadcast paused itself.
*/
const (
	PausedBounces    = "bounce_rate"
	PausedComplaints = "complaint_rate"
)

/*
Tally counts a broadcast's recipients again, stores the counts, pauses
a send that is hurting its address and tells open screens.
*/
func (b *Broadcasts) Tally(ctx context.Context, ws, id ids.UUID) error {
	started := b.Clock.Now()

	err := b.locked(ctx, ws, id, func(ctx context.Context, tx *database.Tx, row models.Broadcast) error {
		q := models.New(tx)

		counts, err := q.TallyBroadcastRecipients(ctx, models.TallyBroadcastRecipientsParams{
			WorkspaceID: ws, BroadcastID: id,
		})
		if err != nil {
			return wrap("tally recipients", err)
		}

		raw, err := json.Marshal(counts)
		if err != nil {
			return wrap("encode counts", err)
		}

		err = q.SetBroadcastCounts(ctx, models.SetBroadcastCountsParams{WorkspaceID: ws, ID: id, Counts: raw})
		if err != nil {
			return wrap("store counts", err)
		}

		row.Counts = raw

		if why := b.harmful(row, counts); why != "" {
			_, err := b.move(ctx, tx, row, move{status: StatusPaused, reason: why})

			return err
		}

		return b.changed(ctx, tx, row)
	})
	if err != nil {
		return gone(err)
	}

	return b.again(ctx, ws, id, started)
}

/*
again counts once more when a recipient changed while this count ran,
since that change's own count was held back by this one.
*/
func (b *Broadcasts) again(ctx context.Context, ws, id ids.UUID, started time.Time) error {
	var last time.Time

	err := b.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		var err error

		last, err = q.LastBroadcastRecipientChange(ctx, models.LastBroadcastRecipientChangeParams{
			WorkspaceID: ws, BroadcastID: id,
		})

		return wrap("last recipient change", err)
	})
	if err != nil || !last.After(started) {
		return err
	}

	return queue.Postpone(time.Second)
}

/*
harmful returns why a sending email broadcast must pause, or "" when it
may go on.
*/
func (b *Broadcasts) harmful(row models.Broadcast, c models.TallyBroadcastRecipientsRow) string {
	if row.Status != StatusSending || row.Channel != ChannelEmail || c.Sent < int64(max(b.Config.PauseAfter, 1)) {
		return ""
	}

	sent := float64(c.Sent)

	switch {
	case float64(c.Complained)/sent > b.Config.ComplaintRate:
		return PausedComplaints
	case float64(c.Bounced)/sent > b.Config.BounceRate:
		return PausedBounces
	}

	return ""
}

/*
changed tells open screens the broadcast moved on.
*/
func (b *Broadcasts) changed(ctx context.Context, tx *database.Tx, row models.Broadcast) error {
	reason := ""
	if row.PauseReason != nil {
		reason = *row.PauseReason
	}

	return b.Events.Dispatch(ctx, tx, appevents.BroadcastUpdated{
		Base:        appevents.Base{ID: ids.New(), WorkspaceID: row.WorkspaceID, OccurredAt: b.Clock.Now()},
		BroadcastID: row.ID, Status: row.Status, PauseReason: reason, Counts: row.Counts,
	})
}
