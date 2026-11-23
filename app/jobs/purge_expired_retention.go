/*
|--------------------------------------------------------------------------
| Purge Expired Retention
|--------------------------------------------------------------------------
|
| The daily round that empties every workspace's trash, deletes its spam
| and removes the stored source of its email, each after the days the
| workspace chose.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/retention"
)

/*
PurgeExpiredRetentionArgs names the job; it takes no arguments.
*/
type PurgeExpiredRetentionArgs struct{}

/*
Kind names the job.
*/
func (PurgeExpiredRetentionArgs) Kind() string { return "purge_expired_retention" }

/*
Options run one round a day, tried three times.
*/
func (PurgeExpiredRetentionArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: 24 * time.Hour}}
}

/*
PurgeExpiredRetention runs the round.
*/
type PurgeExpiredRetention struct {
	Purge *retention.PurgeExpired
}

/*
Handle runs the round.
*/
func (w *PurgeExpiredRetention) Handle(ctx context.Context, _ *queue.Job[PurgeExpiredRetentionArgs]) error {
	_, err := w.Purge.Handle(ctx)

	return err
}
