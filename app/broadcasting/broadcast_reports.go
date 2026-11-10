/*
|--------------------------------------------------------------------------
| Broadcast Reports
|--------------------------------------------------------------------------
|
| A broadcast's report fills in on screen while it sends. Every new
| count and status reaches the workspace's channel, which only members
| hear; the numbers say how many, never who.
|
*/

package broadcasting

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/events"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastReports broadcasts each broadcast's progress to the workspace.
*/
type BroadcastReports struct {
	Server Publisher
}

/*
progress is what a screen learns of a broadcast: which one, its status,
why it paused and its latest counts.
*/
type progress struct {
	ID          string          `json:"id"`
	Object      string          `json:"object"`
	Status      string          `json:"status"`
	PauseReason string          `json:"pause_reason,omitempty"`
	Counts      json.RawMessage `json:"counts,omitempty"`
}

/*
Listen makes the broadcasts follow every committed broadcast update.
*/
func (b *BroadcastReports) Listen(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.BroadcastUpdated) error {
		data := progress{
			ID: models.PublicID(models.PrefixBroadcast, e.BroadcastID), Object: "broadcast", Status: e.Status,
			PauseReason: e.PauseReason, Counts: e.Counts,
		}

		channels := []string{WorkspaceChannel(e.WorkspaceID)}

		return (&Broadcaster{Server: b.Server}).publish(ctx, channels, message(e, data))
	})
}
