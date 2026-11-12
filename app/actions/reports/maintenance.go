/*
|--------------------------------------------------------------------------
| Keeping Facts Fresh
|--------------------------------------------------------------------------
|
| The triggers queue every conversation that changed, two seconds out so
| a burst of changes is worked out once. Refresh takes the due ones
| across workspaces and builds them, then moves the backfills and full
| rebuilds on by a chunk each, so a workspace that predates the log, or
| whose office hours changed, catches up without one long transaction.
| A conversation whose build failed is queued again.
|
*/

package reports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The sizes of the work one refresh does.
*/
const (
	takeBatch      = 500
	buildBatch     = 250
	rebuildChunk   = 2000
	rebuildsAtOnce = 4
)

/*
Maintenance keeps the facts fresh.
*/
type Maintenance struct {
	Builder *Builder
	IDs     *ids.Registry
}

/*
read runs reads inside the workspace.
*/
func (m *Maintenance) read(ctx context.Context, ws ids.UUID, fn func(q *models.Queries) error) error {
	return m.Builder.DB.Transaction(tenancy.With(ctx, ws.String()), func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
Refresh builds the conversations due before the moment and moves the
backfills and rebuilds on, reporting whether work is left.
*/
func (m *Maintenance) Refresh(ctx context.Context, dueBefore time.Time) (bool, error) {
	drained, err := m.Drain(ctx, dueBefore)
	if err != nil {
		return true, err
	}

	more, err := m.Rebuild(ctx)

	return more || drained >= takeBatch, err
}

/*
Drain builds one batch of the conversations due before the moment and
returns how many it took.
*/
func (m *Maintenance) Drain(ctx context.Context, dueBefore time.Time) (int, error) {
	rows, err := models.New(m.Builder.DB).TakeReportPending(ctx, models.TakeReportPendingParams{
		Batch: takeBatch, DueBefore: dueBefore,
	})
	if err != nil {
		return 0, fmt.Errorf("take pending facts: %w", err)
	}

	byWorkspace := map[ids.UUID][]ids.UUID{}
	for _, r := range rows {
		byWorkspace[r.WorkspaceID] = append(byWorkspace[r.WorkspaceID], r.ConversationID)
	}

	var errs []error
	for ws, convs := range byWorkspace {
		if err := m.buildAll(ctx, ws, convs); err != nil {
			errs = append(errs, err, m.requeue(ctx, ws, convs))
		}
	}

	return len(rows), errors.Join(errs...)
}

/*
buildAll builds the conversations in batches.
*/
func (m *Maintenance) buildAll(ctx context.Context, ws ids.UUID, convs []ids.UUID) error {
	for start := 0; start < len(convs); start += buildBatch {
		end := min(start+buildBatch, len(convs))
		if err := m.Builder.Build(ctx, ws, convs[start:end]); err != nil {
			return fmt.Errorf("build facts: %w", err)
		}
	}

	return nil
}

/*
requeue queues conversations again after a failed build.
*/
func (m *Maintenance) requeue(ctx context.Context, ws ids.UUID, convs []ids.UUID) error {
	ctx = tenancy.With(context.WithoutCancel(ctx), ws.String())

	return m.Builder.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).MarkReportPending(ctx, models.MarkReportPendingParams{WorkspaceID: ws, Ids: convs})
	})
}

/*
Rebuild moves each workspace with a backfill or a rebuild on by one
chunk, and reports whether any has more left.
*/
func (m *Maintenance) Rebuild(ctx context.Context) (bool, error) {
	states, err := models.New(m.Builder.DB).ReportRebuilds(ctx, rebuildsAtOnce)
	if err != nil {
		return false, fmt.Errorf("find rebuilds: %w", err)
	}

	for _, s := range states {
		step := m.rebuildStep
		if !s.Backfilled {
			step = m.backfill
		}

		if err := step(ctx, s.WorkspaceID); err != nil {
			return true, err
		}
	}

	return len(states) > 0, nil
}

/*
rebuildStep builds the next chunk of a workspace's conversations, and
ends the rebuild when none are left.
*/
func (m *Maintenance) rebuildStep(ctx context.Context, ws ids.UUID) error {
	var chunk []ids.UUID

	err := m.read(ctx, ws, func(q *models.Queries) error {
		state, err := q.FindReportState(ctx, ws)
		if err != nil {
			return fmt.Errorf("find report state: %w", err)
		}

		after := ids.Nil()
		if state.RebuildCursor != nil {
			after = *state.RebuildCursor
		}

		chunk, err = q.ReportRebuildChunk(ctx, models.ReportRebuildChunkParams{
			WorkspaceID: ws, After: after, Batch: rebuildChunk,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("read rebuild chunk: %w", err)
	}

	if err := m.buildAll(ctx, ws, chunk); err != nil {
		return err
	}

	var cursor *ids.UUID
	if len(chunk) == rebuildChunk {
		cursor = &chunk[len(chunk)-1]
	}

	return m.read(ctx, ws, func(q *models.Queries) error {
		return q.SetReportRebuildCursor(ctx, models.SetReportRebuildCursorParams{
			Cursor: cursor, Now: m.Builder.Clock.Now(), WorkspaceID: ws,
		})
	})
}

/*
RebuildNow runs a workspace's backfill and rebuild to the end, for the
seeder and tests.
*/
func (m *Maintenance) RebuildNow(ctx context.Context, ws ids.UUID) error {
	for range 100_000 {
		var state models.ReportState

		err := m.read(ctx, ws, func(q *models.Queries) error {
			var err error
			state, err = q.FindReportState(ctx, ws)

			return err
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find report state: %w", err)
		}

		switch {
		case state.BackfilledAt == nil:
			err = m.backfill(ctx, ws)
		case state.RebuildRequestedAt != nil:
			err = m.rebuildStep(ctx, ws)
		default:
			return nil
		}

		if err != nil {
			return err
		}
	}

	return nil
}
