/*
|--------------------------------------------------------------------------
| Purge Stale Drafts
|--------------------------------------------------------------------------
|
| A draft nobody touched for the retention period is forgotten, so it is
| deleted once a day, a batch at a time. Files it held become orphans and
| leave with the next orphan purge.
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
PurgeStaleDraftsArgs names the job; it takes no arguments.
*/
type PurgeStaleDraftsArgs struct{}

/*
Kind names the job.
*/
func (PurgeStaleDraftsArgs) Kind() string {
	return "purge_stale_drafts"
}

/*
Options run the job once a day at most, and retry it a few times.
*/
func (PurgeStaleDraftsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}

/*
PurgeStaleDrafts deletes drafts untouched for RetentionDays days; 0
keeps them for ever.
*/
type PurgeStaleDrafts struct {
	DB            *database.DB
	Clock         clock.Clock
	RetentionDays int
}

/*
Handle deletes the stale drafts.
*/
func (w *PurgeStaleDrafts) Handle(ctx context.Context, _ *queue.Job[PurgeStaleDraftsArgs]) error {
	if w.RetentionDays <= 0 {
		return nil
	}

	cutoff := w.Clock.Now().Add(-time.Duration(w.RetentionDays) * day)

	return prune(ctx, func(ctx context.Context) (int64, error) {
		return models.New(w.DB).PurgeStaleDrafts(ctx, models.PurgeStaleDraftsParams{Cutoff: cutoff, Batch: pruneBatch})
	})
}
