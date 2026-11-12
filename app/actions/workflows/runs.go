/*
|--------------------------------------------------------------------------
| Workflow Runs
|--------------------------------------------------------------------------
|
| Every member who may read workflows may see how they ran: the newest
| runs of a workflow, each with the trace of the steps it executed, the
| path it took and the few facts each step kept. Traces never hold
| message or request bodies, so they are safe to show to the team.
|
*/

package workflows

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxRunsListed bounds one page of runs.
*/
const maxRunsListed = 50

/*
RunTrace is one run with the steps it executed, in order.
*/
type RunTrace struct {
	Run   models.WorkflowRun
	Steps []models.WorkflowRunStep
}

/*
Runs lists a workflow's newest runs with their traces.
*/
func (w *Workflows) Runs(ctx context.Context, m authz.Membership, id ids.UUID, limit int) ([]RunTrace, error) {
	if !m.Can("workflows:read") {
		return nil, exceptions.ErrForbidden
	}

	rows := int32(20)
	if limit > 0 && limit <= maxRunsListed {
		rows = int32(limit)
	}

	var out []RunTrace
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		runs, err := q.ListWorkflowRuns(ctx, models.ListWorkflowRunsParams{
			WorkspaceID: m.Workspace.ID, WorkflowID: id, RowLimit: rows,
		})
		if err != nil {
			return fmt.Errorf("list workflow runs: %w", err)
		}

		for _, run := range runs {
			steps, err := q.ListWorkflowRunSteps(ctx, models.ListWorkflowRunStepsParams{
				WorkspaceID: m.Workspace.ID, RunID: run.ID,
			})
			if err != nil {
				return fmt.Errorf("list run steps: %w", err)
			}

			out = append(out, RunTrace{Run: run, Steps: steps})
		}

		return nil
	})

	return out, err
}
