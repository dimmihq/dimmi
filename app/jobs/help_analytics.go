/*
|--------------------------------------------------------------------------
| Pruning Help Center Counts
|--------------------------------------------------------------------------
|
| Every night the help centers' counts of views and searches older than
| they are kept are pruned, across every workspace.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
HelpAnalyticsPruner drops help center counts older than they are kept.
*/
type HelpAnalyticsPruner interface {
	/*
		Prune drops the old counts.
	*/
	Prune(ctx context.Context) error
}

/*
PruneHelpAnalyticsArgs names the job; it takes no arguments.
*/
type PruneHelpAnalyticsArgs struct{}

/*
Kind names the job.
*/
func (PruneHelpAnalyticsArgs) Kind() string {
	return "prune_help_analytics"
}

/*
Options try the prune a few times; tomorrow's run catches up anyway.
*/
func (PruneHelpAnalyticsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3}
}

/*
PruneHelpAnalytics drops help center counts older than they are kept.
*/
type PruneHelpAnalytics struct {
	Analytics HelpAnalyticsPruner
}

/*
Handle prunes the counts.
*/
func (w *PruneHelpAnalytics) Handle(ctx context.Context, _ *queue.Job[PruneHelpAnalyticsArgs]) error {
	return w.Analytics.Prune(ctx)
}
