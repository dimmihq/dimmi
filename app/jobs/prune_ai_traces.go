/*
|--------------------------------------------------------------------------
| Prune AI Traces
|--------------------------------------------------------------------------
|
| A run's steps carry the payloads that cost the most to keep, the
| redacted prompts and replies, so those payloads are dropped once
| they pass PayloadRetentionDays while the step rows and their numbers
| stay. Playground runs go after PlaygroundRetentionDays, and every run,
| with its steps, after RunsRetentionDays. The round runs once a day,
| one workspace at a time inside that workspace's transaction, a batch
| at a time, and skips a workspace whose records are held.
|
*/

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
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
PruneAiTraces drops step payloads past PayloadRetentionDays, playground
runs past PlaygroundRetentionDays and every run past
RunsRetentionDays; a period of 0 keeps that data for ever. Retention
decides which workspaces' traces may go.
*/
type PruneAiTraces struct {
	DB                      *database.DB
	Clock                   clock.Clock
	Retention               contracts.RetentionGuard
	PayloadRetentionDays    int
	PlaygroundRetentionDays int
	RunsRetentionDays       int
}

/*
Handle prunes every workspace's traces.
*/
func (w *PruneAiTraces) Handle(ctx context.Context, _ *queue.Job[PruneAiTracesArgs]) error {
	workspaces, err := models.New(w.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to prune: %w", err)
	}

	for _, workspace := range workspaces {
		if err := w.pruneWorkspace(ctx, workspace.ID); err != nil {
			return err
		}
	}

	return nil
}

/*
pruneWorkspace prunes one workspace's traces, unless the retention
guard holds them.
*/
func (w *PruneAiTraces) pruneWorkspace(ctx context.Context, workspaceID ids.UUID) error {
	if w.Retention != nil {
		subject := contracts.RetentionSubject{Kind: contracts.RetentionWorkspace, ID: workspaceID}
		ok, err := w.Retention.MayDestroy(ctx, workspaceID, subject)
		if err != nil {
			return fmt.Errorf("ask retention guard: %w", err)
		}
		if !ok {
			return nil
		}
	}

	scoped := tenancy.With(ctx, workspaceID.String())

	dropPayloads := func(ctx context.Context, q *models.Queries, cutoff time.Time) (int64, error) {
		return q.NullExpiredRunStepPayloads(ctx, models.NullExpiredRunStepPayloadsParams{
			WorkspaceID: workspaceID, Cutoff: cutoff, Batch: pruneBatch,
		})
	}
	if err := w.batches(scoped, w.PayloadRetentionDays, dropPayloads); err != nil {
		return fmt.Errorf("drop step payloads: %w", err)
	}

	if err := w.pruneRuns(scoped, workspaceID, "playground", w.PlaygroundRetentionDays); err != nil {
		return err
	}

	return w.pruneRuns(scoped, workspaceID, "", w.RunsRetentionDays)
}

/*
pruneRuns deletes the workspace's runs of the kind, every kind when
empty, older than the period.
*/
func (w *PruneAiTraces) pruneRuns(ctx context.Context, workspaceID ids.UUID, kind string, days int) error {
	err := w.batches(ctx, days, func(ctx context.Context, q *models.Queries, cutoff time.Time) (int64, error) {
		return q.PruneAiRuns(ctx, models.PruneAiRunsParams{
			WorkspaceID: workspaceID, Cutoff: cutoff, Kind: kind, Batch: pruneBatch,
		})
	})
	if err != nil {
		return fmt.Errorf("prune runs: %w", err)
	}
	return nil
}

/*
batches runs one batch statement per transaction against the cutoff
the period gives, until a batch comes back short. A period of 0 or
less runs nothing.
*/
func (w *PruneAiTraces) batches(ctx context.Context, days int,
	batch func(ctx context.Context, q *models.Queries, cutoff time.Time) (int64, error),
) error {
	if days <= 0 {
		return nil
	}

	cutoff := w.Clock.Now().Add(-time.Duration(days) * day)

	return prune(ctx, func(ctx context.Context) (int64, error) {
		var n int64
		err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error
			n, err = batch(ctx, models.New(tx), cutoff)
			return err
		})
		return n, err
	})
}
