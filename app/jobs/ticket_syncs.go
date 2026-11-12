/*
|--------------------------------------------------------------------------
| Ticket Syncs
|--------------------------------------------------------------------------
|
| The periodic checks that keep tickets current: linked issues re-read
| in case a provider's webhook was missed, and new conversations looked
| over for a spike about one thing.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
SyncTicketIssuesArgs re-checks linked issues not synced lately.
*/
type SyncTicketIssuesArgs struct{}

/*
DetectIncidentSpikesArgs looks for spikes of new conversations.
*/
type DetectIncidentSpikesArgs struct{}

/*
SyncTicketIssues re-checks linked issues.
*/
type SyncTicketIssues struct {
	Issues Sweeper
}

/*
Handle runs the sweep.
*/
func (w *SyncTicketIssues) Handle(ctx context.Context, _ *queue.Job[SyncTicketIssuesArgs]) error {
	return w.Issues.Sweep(ctx)
}

/*
DetectIncidentSpikes looks for spikes.
*/
type DetectIncidentSpikes struct {
	Spikes Sweeper
}

/*
Handle runs the detector.
*/
func (w *DetectIncidentSpikes) Handle(ctx context.Context, _ *queue.Job[DetectIncidentSpikesArgs]) error {
	return w.Spikes.Sweep(ctx)
}
