/*
|--------------------------------------------------------------------------
| Sync Runs
|--------------------------------------------------------------------------
|
| A source runs one sync at a time: asking again while one runs returns
| that run. A run starts in the transaction that asked for it, so it is
| never lost, and canceling it stops its remaining jobs at their next
| step while what it already indexed stays. Members who may sync, and
| not only admins, can start and cancel runs.
|
*/

package knowledgesources

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Sync starts a run of the source now, or returns the one running.
*/
func (e *Engine) Sync(ctx context.Context, m authz.Membership, id ids.UUID) (models.KnowledgeSyncRun, error) {
	if !m.Can("knowledge:sync") {
		return models.KnowledgeSyncRun{}, exceptions.ErrForbidden
	}

	var out models.KnowledgeSyncRun

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		s, err := q.LockKnowledgeSource(ctx, models.LockKnowledgeSourceParams{WorkspaceID: m.Workspace.ID, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return ErrSourceNotFound
		}

		if err != nil {
			return fmt.Errorf("lock knowledge source: %w", err)
		}

		out, err = e.start(ctx, tx, s, "manual")

		return err
	})

	return out, err
}

/*
start begins a run of the locked source unless one is running, which
it returns instead.
*/
func (e *Engine) start(ctx context.Context, tx *database.Tx, s models.KnowledgeSource, trigger string,
) (models.KnowledgeSyncRun, error) {
	q := models.New(tx)

	running, err := q.FindActiveKnowledgeSyncRun(ctx, models.FindActiveKnowledgeSyncRunParams{
		WorkspaceID: s.WorkspaceID, SourceID: s.ID,
	})
	if err == nil {
		return running, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return running, fmt.Errorf("find running sync: %w", err)
	}

	now := e.Clock.Now()

	run, err := q.CreateKnowledgeSyncRun(ctx, models.CreateKnowledgeSyncRunParams{
		ID: ids.NewV7At(now), WorkspaceID: s.WorkspaceID, SourceID: s.ID, Trigger: trigger, FullRun: true, Now: &now,
	})
	if err != nil {
		return run, fmt.Errorf("start sync: %w", err)
	}

	if err := q.SetKnowledgeSourceState(ctx, models.SetKnowledgeSourceStateParams{
		Status: "syncing", LastErrorCode: s.LastErrorCode, LastError: s.LastError, Now: now,
		WorkspaceID: s.WorkspaceID, ID: s.ID,
	}); err != nil {
		return run, fmt.Errorf("mark source syncing: %w", err)
	}

	args := jobs.DiscoverKnowledgeSourceArgs{WorkspaceID: s.WorkspaceID, RunID: run.ID}

	return run, wrap("queue discovery", e.Queue.PushTx(ctx, tx, args))
}

/*
Cancel stops a running sync; what it indexed stays.
*/
func (e *Engine) Cancel(ctx context.Context, m authz.Membership, sourceID, runID ids.UUID,
) (models.KnowledgeSyncRun, error) {
	if !m.Can("knowledge:sync") {
		return models.KnowledgeSyncRun{}, exceptions.ErrForbidden
	}

	var out models.KnowledgeSyncRun

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		run, err := findRun(ctx, q, m.Workspace.ID, runID)
		if err != nil || run.SourceID != sourceID {
			return cmpErr(err, ErrRunNotFound)
		}

		out = run
		if run.Status != "running" && run.Status != "queued" {
			return nil
		}

		out, err = e.end(ctx, tx, run, outcome{status: "canceled"})

		return err
	})

	return out, err
}

/*
Runs returns a source's latest runs, newest first.
*/
func (e *Engine) Runs(ctx context.Context, m authz.Membership, sourceID ids.UUID) ([]models.KnowledgeSyncRun, error) {
	if !m.Can("settings:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.KnowledgeSyncRun

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		if _, err := findSource(ctx, q, m.Workspace.ID, sourceID); err != nil {
			return err
		}

		var err error

		out, err = q.ListKnowledgeSyncRuns(ctx, models.ListKnowledgeSyncRunsParams{
			WorkspaceID: m.Workspace.ID, SourceID: sourceID, RowLimit: keptRuns,
		})

		return wrap("list sync runs", err)
	})

	return out, err
}

/*
findRun returns a run of the workspace, or 404.
*/
func findRun(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.KnowledgeSyncRun, error) {
	run, err := q.FindKnowledgeSyncRun(ctx, models.FindKnowledgeSyncRunParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return run, ErrRunNotFound
	}

	return run, wrap("find sync run", err)
}

/*
cmpErr is the error, or the fallback when there is none.
*/
func cmpErr(err, fallback error) error {
	if err != nil {
		return err
	}

	return fallback
}
