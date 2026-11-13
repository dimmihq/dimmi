/*
|--------------------------------------------------------------------------
| Ticket Syncs
|--------------------------------------------------------------------------
|
| The periodic check that keeps tickets current: linked issues re-read
| in case a provider's webhook was missed.
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
