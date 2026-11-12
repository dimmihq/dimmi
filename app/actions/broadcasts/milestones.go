/*
|--------------------------------------------------------------------------
| Broadcast Milestones
|--------------------------------------------------------------------------
|
| A broadcast that finishes sending, or pauses, dispatches an event of
| its own in the transaction of the move, beside the update open screens
| hear, so webhooks learn of it once rather than on every new count.
|
*/

package broadcasts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
milestone dispatches broadcast.sent or broadcast.paused when a move
just reached one, caused by the member who moved it or the system.
*/
func (b *Broadcasts) milestone(ctx context.Context, tx *database.Tx, row models.Broadcast, by *ids.UUID) error {
	if row.Status != StatusSent && row.Status != StatusPaused {
		return nil
	}

	actor := appevents.Actor{Type: "system", ID: ids.Nil()}
	if by != nil {
		actor = appevents.Actor{Type: "member", ID: *by}
	}

	base := appevents.Base{ID: ids.New(), WorkspaceID: row.WorkspaceID, OccurredAt: b.Clock.Now(), Actor: actor}
	if row.Status == StatusSent {
		return wrap("dispatch broadcast sent", b.Events.Dispatch(ctx, tx, appevents.BroadcastSent{
			Base: base, BroadcastID: row.ID,
		}))
	}

	reason := ""
	if row.PauseReason != nil {
		reason = *row.PauseReason
	}

	return wrap("dispatch broadcast paused", b.Events.Dispatch(ctx, tx, appevents.BroadcastPaused{
		Base: base, BroadcastID: row.ID, Reason: reason,
	}))
}
