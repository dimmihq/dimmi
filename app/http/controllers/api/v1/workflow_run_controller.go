/*
|--------------------------------------------------------------------------
| Workflow Runs Controller
|--------------------------------------------------------------------------
|
| How a workflow ran: its newest runs, each with its step trace.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowRunsOutput is a workflow's runs.
*/
type WorkflowRunsOutput struct {
	Body fhttp.Collection[resources.WorkflowRun]
}

/*
Runs lists a workflow's newest runs with their traces.
*/
func (c *WorkflowController) Runs(ctx context.Context, in *requests.WorkflowRunsRequest) (*WorkflowRunsOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	rows, err := c.Workflows.Runs(ctx, membership(ctx), id, in.Limit)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.WorkflowRun]{Items: resources.NewWorkflowRuns(rows), Limit: 50}

	return &WorkflowRunsOutput{Body: fhttp.NewCollection(page)}, nil
}
