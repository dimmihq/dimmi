/*
|--------------------------------------------------------------------------
| Prune Audit Logs
|--------------------------------------------------------------------------
|
| Audit entries older than the retention period are deleted once a day,
| a batch at a time so no single statement holds locks for long. This
| job is the only thing that ever deletes an audit entry; with no
| retention period, it deletes nothing.
|
*/

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/models"
)

/*
pruneBatch is how many rows one delete removes, pruneAttempts how often
a prune is tried before the next day's run takes over, and day the
period retention is counted in.
*/
const (
	pruneBatch    = 10_000
	pruneAttempts = 3
	day           = 24 * time.Hour
)

/*
PruneAuditLogsArgs names the job; it takes no arguments.
*/
type PruneAuditLogsArgs struct{}

/*
Kind names the job.
*/
func (PruneAuditLogsArgs) Kind() string {
	return "prune_audit_logs"
}

/*
Options run the job once a day at most, and retry it a few times.
*/
func (PruneAuditLogsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}

/*
PruneAuditLogs deletes audit entries older than RetentionDays days; 0
keeps them for ever.
*/
type PruneAuditLogs struct {
	DB            *database.DB
	Clock         clock.Clock
	RetentionDays int
}

/*
Handle deletes the expired entries.
*/
func (w *PruneAuditLogs) Handle(ctx context.Context, _ *queue.Job[PruneAuditLogsArgs]) error {
	if w.RetentionDays <= 0 {
		return nil
	}

	cutoff := w.Clock.Now().Add(-time.Duration(w.RetentionDays) * day)

	return prune(ctx, func(ctx context.Context) (int64, error) {
		return models.New(w.DB).PruneAuditLogs(ctx, models.PruneAuditLogsParams{Cutoff: cutoff, Batch: pruneBatch})
	})
}

/*
prune runs the batch delete until a batch comes back short.
*/
func prune(ctx context.Context, batch func(ctx context.Context) (int64, error)) error {
	for {
		deleted, err := batch(ctx)
		if err != nil {
			return fmt.Errorf("prune: %w", err)
		}

		if deleted < pruneBatch {
			return nil
		}
	}
}
