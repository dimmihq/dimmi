/*
|--------------------------------------------------------------------------
| Applying Workflow And Series Changes
|--------------------------------------------------------------------------
|
| A workflow's or a series' working copy changes through the actions its
| builder saves with; what runs changes only when a teammate publishes.
| Revert puts the working copy back while it still holds what Operator
| wrote.
|
*/

package operator

import (
	"context"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
applyAutomation applies a workflow or series operation.
*/
func (e *Engine) applyAutomation(ctx context.Context, m authz.Membership, op *Operation) error {
	if op.Kind == KindWorkflowSet {
		return e.saveWorkflow(ctx, m, op)
	}
	return e.saveSeries(ctx, m, op)
}

/*
saveWorkflow writes a workflow's working copy, keeping what it replaced.
*/
func (e *Engine) saveWorkflow(ctx context.Context, m authz.Membership, op *Operation) error {
	wf, err := e.Services.Workflows.Show(ctx, m, *op.Workflow)
	if err != nil {
		return err
	}
	after, err := e.Services.Workflows.Update(ctx, workflows.UpdateInput{
		Membership: m, ID: wf.ID, LockVersion: wf.LockVersion, Spec: op.Definition,
	})
	op.Before, op.After = wf.Draft, after.Draft
	return err
}

/*
saveSeries writes a series' working copy, keeping its settings and what
it replaced.
*/
func (e *Engine) saveSeries(ctx context.Context, m authz.Membership, op *Operation) error {
	row, err := e.Services.Series.Show(ctx, m, *op.Series)
	if err != nil {
		return err
	}
	spec, err := readSeries(op.Definition)
	if err != nil {
		return err
	}
	in := series.CurrentSettings(row)
	in.Membership, in.Spec = m, &spec
	after, err := e.Services.Series.Save(ctx, in)
	op.Before, op.After = row.Working, after.Working
	return err
}

/*
revertAutomation undoes a workflow or series operation, reporting false
when it moved on since.
*/
func (e *Engine) revertAutomation(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	if op.Kind == KindWorkflowSet {
		return e.restoreWorkflow(ctx, m, op)
	}
	return e.restoreSeries(ctx, m, op)
}

/*
restoreWorkflow puts a workflow's working copy back while it still
holds what Operator wrote.
*/
func (e *Engine) restoreWorkflow(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	wf, err := e.Services.Workflows.Show(ctx, m, *op.Workflow)
	if err != nil || wf.DeletedAt != nil || !sameJSON(wf.Draft, op.After) {
		return false, err
	}
	_, err = e.Services.Workflows.Update(ctx, workflows.UpdateInput{
		Membership: m, ID: wf.ID, LockVersion: wf.LockVersion, Spec: op.Before,
	})
	return err == nil, err
}

/*
restoreSeries puts a series' working copy back while it still holds
what Operator wrote.
*/
func (e *Engine) restoreSeries(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	row, err := e.Services.Series.Show(ctx, m, *op.Series)
	if err != nil || row.Status == series.StatusArchived || !sameJSON(row.Working, op.After) {
		return false, err
	}
	spec, err := readSeries(op.Before)
	if err != nil {
		return false, err
	}
	in := series.CurrentSettings(row)
	in.Membership, in.Spec = m, &spec
	_, err = e.Services.Series.Save(ctx, in)
	return err == nil, err
}
