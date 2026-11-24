/*
|--------------------------------------------------------------------------
| Workflow Runs
|--------------------------------------------------------------------------
|
| Every member who may read workflows may see how they ran: the newest
| runs of a workflow, each with the trace of the steps it executed, the
| path it took and the few facts each step kept. Traces never hold
| message or request bodies, so they are safe to show to the team. Each
| run names the version that handled it.
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
	Run           models.WorkflowRun
	Steps         []models.WorkflowRunStep
	VersionNumber int32
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

		out, err = traces(ctx, q, runs)

		return err
	})

	return out, err
}

/*
traces reads each run's steps and names the version that handled it.
*/
func traces(ctx context.Context, q *models.Queries, runs []models.WorkflowRun) ([]RunTrace, error) {
	if len(runs) == 0 {
		return nil, nil
	}

	ws := runs[0].WorkspaceID
	numbers, err := versionNumbers(ctx, q, ws, runs[0].WorkflowID)
	if err != nil {
		return nil, err
	}

	out := make([]RunTrace, 0, len(runs))
	for _, run := range runs {
		steps, err := q.ListWorkflowRunSteps(ctx, models.ListWorkflowRunStepsParams{WorkspaceID: ws, RunID: run.ID})
		if err != nil {
			return nil, fmt.Errorf("list run steps: %w", err)
		}

		trace := RunTrace{Run: run, Steps: steps}
		if run.VersionID != nil {
			trace.VersionNumber = numbers[*run.VersionID]
		}

		out = append(out, trace)
	}

	return out, nil
}

/*
versionNumbers maps a workflow's version ids to their numbers.
*/
func versionNumbers(ctx context.Context, q *models.Queries, ws, id ids.UUID) (map[ids.UUID]int32, error) {
	versions, err := q.ListWorkflowVersions(ctx, models.ListWorkflowVersionsParams{
		WorkspaceID: ws, WorkflowID: id, RowLimit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("list workflow versions: %w", err)
	}

	out := make(map[ids.UUID]int32, len(versions))
	for _, v := range versions {
		out[v.ID] = v.Number
	}

	return out, nil
}
