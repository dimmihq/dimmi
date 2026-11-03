/*
|--------------------------------------------------------------------------
| Prune Security Events
|--------------------------------------------------------------------------
|
| A person's security log keeps a year of sign-ins and account changes,
| enough to look back on anything suspicious, and no more: older entries
| are deleted once a day, a batch at a time.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/models"
)

/*
PruneSecurityEventsArgs names the job; it takes no arguments.
*/
type PruneSecurityEventsArgs struct{}

/*
Kind names the job.
*/
func (PruneSecurityEventsArgs) Kind() string {
	return "prune_security_events"
}

/*
Options run the job once a day at most, and retry it a few times.
*/
func (PruneSecurityEventsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}

/*
PruneSecurityEvents deletes security log entries older than
RetentionDays days; 0 keeps them for ever.
*/
type PruneSecurityEvents struct {
	DB            *database.DB
	Clock         clock.Clock
	RetentionDays int
}

/*
Handle deletes the expired entries.
*/
func (w *PruneSecurityEvents) Handle(ctx context.Context, _ *queue.Job[PruneSecurityEventsArgs]) error {
	if w.RetentionDays <= 0 {
		return nil
	}

	cutoff := w.Clock.Now().Add(-time.Duration(w.RetentionDays) * day)

	return prune(ctx, func(ctx context.Context) (int64, error) {
		return models.New(w.DB).PruneSecurityEvents(ctx, models.PruneSecurityEventsParams{
			Cutoff: cutoff, Batch: pruneBatch,
		})
	})
}
