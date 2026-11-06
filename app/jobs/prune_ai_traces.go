/*
|--------------------------------------------------------------------------
| Prune AI Traces
|--------------------------------------------------------------------------
|
| A run's steps carry the payloads that cost the most to keep — the
| prompts and transcripts — so they go at TracesRetentionDays, while
| the run rows themselves last RunsRetentionDays for the inbox's why
| panel. Both delete oldest first, a batch at a time.
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
PruneAiTracesArgs names the job; it takes no arguments.
*/
type PruneAiTracesArgs struct{}

/*
Kind names the job.
*/
func (PruneAiTracesArgs) Kind() string { return "prune_ai_traces" }

/*
Options run the job once a day at most, and retry it a few times.
*/
func (PruneAiTracesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}

/*
PruneAiTraces deletes steps past StepsRetentionDays and runs past
RunsRetentionDays; either 0 keeps its table for ever.
*/
type PruneAiTraces struct {
	DB                 *database.DB
	Clock              clock.Clock
	StepsRetentionDays int
	RunsRetentionDays  int
}

/*
Handle prunes both tables.
*/
func (w *PruneAiTraces) Handle(ctx context.Context, _ *queue.Job[PruneAiTracesArgs]) error {
	if err := w.pruneSteps(ctx); err != nil {
		return err
	}
	return w.pruneRuns(ctx)
}

/*
pruneSteps deletes step rows older than the steps' retention.
*/
func (w *PruneAiTraces) pruneSteps(ctx context.Context) error {
	if w.StepsRetentionDays <= 0 {
		return nil
	}
	cutoff := w.Clock.Now().Add(-time.Duration(w.StepsRetentionDays) * day)
	return prune(ctx, func(ctx context.Context) (int64, error) {
		return models.New(w.DB).PruneAiRunSteps(ctx, models.PruneAiRunStepsParams{
			Cutoff: cutoff, Batch: pruneBatch,
		})
	})
}

/*
pruneRuns deletes run rows older than the runs' retention.
*/
func (w *PruneAiTraces) pruneRuns(ctx context.Context) error {
	if w.RunsRetentionDays <= 0 {
		return nil
	}
	cutoff := w.Clock.Now().Add(-time.Duration(w.RunsRetentionDays) * day)
	return prune(ctx, func(ctx context.Context) (int64, error) {
		return models.New(w.DB).PruneAiRuns(ctx, models.PruneAiRunsParams{
			Cutoff: cutoff, Batch: pruneBatch,
		})
	})
}
