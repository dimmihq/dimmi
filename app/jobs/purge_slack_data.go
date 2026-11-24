/*
|--------------------------------------------------------------------------
| Purge Slack Data
|--------------------------------------------------------------------------
|
| The daily round that deletes what Slack brought in for every Slack
| connection that ended, by an uninstall, revoked tokens or a teammate's
| disconnect, longer ago than the grace period.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/retention"
)

/*
PurgeSlackDataArgs names the job; it takes no arguments.
*/
type PurgeSlackDataArgs struct{}

/*
Kind names the job.
*/
func (PurgeSlackDataArgs) Kind() string { return "purge_slack_data" }

/*
Options run one round a day, tried three times; the next round carries
on with anything left.
*/
func (PurgeSlackDataArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}

/*
PurgeSlackData runs the round.
*/
type PurgeSlackData struct {
	Purge *retention.PurgeSlackData
}

/*
Handle runs the round.
*/
func (w *PurgeSlackData) Handle(ctx context.Context, _ *queue.Job[PurgeSlackDataArgs]) error {
	_, err := w.Purge.Handle(ctx)

	return err
}
