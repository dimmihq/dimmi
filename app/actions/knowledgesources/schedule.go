/*
|--------------------------------------------------------------------------
| Scheduled Syncs
|--------------------------------------------------------------------------
|
| Every few minutes the sources whose next sync came are started,
| workspace by workspace, a bounded number at a time so a backlog drains
| over a few rounds instead of flooding the queue. A source already
| syncing is left to finish. Runs that stopped moving for half an hour, a worker
| lost mid-run, are closed as stalled so the source can run again.
|
*/

package knowledgesources

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
dueBatch is how many sources one round starts.
*/
const dueBatch = 100

/*
ScheduleDue starts the runs whose time came, workspace by workspace.
*/
func (e *Engine) ScheduleDue(ctx context.Context, _ *queue.Job[jobs.ScheduleKnowledgeSyncsArgs]) error {
	workspaces, err := models.New(e.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to schedule: %w", err)
	}

	for _, w := range workspaces {
		var due []models.ListDueKnowledgeSourcesRow

		err := e.inWorkspace(ctx, w.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
			var err error

			due, err = q.ListDueKnowledgeSources(ctx, models.ListDueKnowledgeSourcesParams{
				WorkspaceID: w.ID, Now: new(e.Clock.Now()), RowLimit: dueBatch,
			})

			return err
		})
		if err != nil {
			return fmt.Errorf("list due sources: %w", err)
		}

		for _, d := range due {
			if err := e.startScheduled(ctx, d); err != nil {
				return err
			}
		}
	}

	return nil
}

/*
startScheduled starts one source's scheduled run, closing a stalled run
first.
*/
func (e *Engine) startScheduled(ctx context.Context, d models.ListDueKnowledgeSourcesRow) error {
	return e.inWorkspace(ctx, d.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		s, err := q.LockKnowledgeSource(ctx, models.LockKnowledgeSourceParams(d))
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("lock due source: %w", err)
		}

		running, err := q.FindActiveKnowledgeSyncRun(ctx, models.FindActiveKnowledgeSyncRunParams{
			WorkspaceID: s.WorkspaceID, SourceID: s.ID,
		})
		if err == nil && e.Clock.Now().Sub(running.UpdatedAt) > stalledAfter {
			if _, err := e.end(ctx, tx, running, outcome{
				status: "failed", code: "stalled", message: "The sync stopped making progress.",
			}); err != nil {
				return err
			}
		}

		_, err = e.start(ctx, tx, s, "schedule")

		return err
	})
}
