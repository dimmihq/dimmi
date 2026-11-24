/*
|--------------------------------------------------------------------------
| Workflow Analytics Controller
|--------------------------------------------------------------------------
|
| How a workflow did over a range of days, for every version or one, and
| the runs that failed, a page at a time.
|
*/

package v1

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowMetricsOutput is a workflow's numbers.
*/
type WorkflowMetricsOutput struct {
	Body fhttp.Resource[resources.WorkflowMetrics]
}

/*
WorkflowFailuresOutput is a page of a workflow's failed runs.
*/
type WorkflowFailuresOutput struct {
	Body fhttp.Collection[resources.WorkflowFailure]
}

/*
Metrics answers a workflow's numbers over a range.
*/
func (c *WorkflowController) Metrics(ctx context.Context, in *requests.WorkflowMetricsRequest,
) (*WorkflowMetricsOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	from, to := dayRange(in.From, in.To)
	m, err := c.Workflows.Metrics(ctx, membership(ctx), id, workflows.MetricsScope{
		From: from, To: to, Version: in.Version,
	})
	if err != nil {
		return nil, err
	}

	return &WorkflowMetricsOutput{Body: fhttp.NewResource(resources.NewWorkflowMetrics(id, m))}, nil
}

/*
Failures lists a workflow's failed runs, newest first.
*/
func (c *WorkflowController) Failures(ctx context.Context, in *requests.WorkflowFailuresRequest,
) (*WorkflowFailuresOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	limit := in.Limit
	if limit == 0 {
		limit = 25
	}

	scope := workflows.FailuresScope{StepID: in.StepID, Limit: limit}
	scope.From, scope.To = dayRange(in.From, in.To)

	if in.Before != "" {
		before, err := c.decode(models.PrefixWorkflowRun, in.Before)
		if err != nil {
			return nil, err
		}

		scope.Before = &before
	}

	rows, err := c.Workflows.Failures(ctx, membership(ctx), id, scope)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.WorkflowFailure]{Items: resources.NewWorkflowFailures(rows), Limit: limit}
	if len(rows) == limit {
		page.NextCursor = models.PublicID(models.PrefixWorkflowRun, rows[len(rows)-1].ID)
	}

	return &WorkflowFailuresOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
dayRange reads a range of whole days, to included; unset ends are
zero.
*/
func dayRange(from, to string) (time.Time, time.Time) {
	start, end := day(from), day(to)
	if !end.IsZero() {
		end = end.Add(24 * time.Hour)
	}

	return start, end
}

/*
WorkflowsReportOutput is every workflow's numbers over a range.
*/
type WorkflowsReportOutput struct {
	Body fhttp.Collection[resources.WorkflowReportRow]
}

/*
Report lists every workflow that started a run in the range with its
numbers.
*/
func (c *WorkflowController) Report(ctx context.Context, in *requests.WorkflowsReportRequest,
) (*WorkflowsReportOutput, error) {
	from, to := dayRange(in.From, in.To)
	rows, err := c.Workflows.Report(ctx, membership(ctx), from, to)
	if err != nil {
		return nil, err
	}

	items := make([]resources.WorkflowReportRow, len(rows))
	for i, r := range rows {
		items[i] = resources.NewWorkflowReportRow(r)
	}

	return &WorkflowsReportOutput{
		Body: fhttp.NewCollection(pagination.Page[resources.WorkflowReportRow]{Items: items, Limit: 500}),
	}, nil
}
