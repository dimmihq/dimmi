/*
|--------------------------------------------------------------------------
| Applying Workflow And Series Changes
|--------------------------------------------------------------------------
|
| A workflow's or a series' working copy changes through the actions its
| builder saves with; what runs changes only when a teammate publishes.
| A new draft Operator suggested is created on accept. Revert puts the
| working copy back while it still holds what Operator wrote, and takes
| a new draft away while it never went live: a workflow is archived,
| a series draft deleted.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
applyAutomation applies a workflow or series operation.
*/
func (e *Engine) applyAutomation(ctx context.Context, m authz.Membership, op *Operation) error {
	switch op.Kind {
	case KindWorkflowSet:
		return e.saveWorkflow(ctx, m, op)
	case KindWorkflowNew:
		return e.createWorkflow(ctx, m, op)
	case KindSeriesSet:
		return e.saveSeries(ctx, m, op)
	default:
		return e.createSeries(ctx, m, op)
	}
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
createWorkflow creates the draft workflow Operator suggested.
*/
func (e *Engine) createWorkflow(ctx context.Context, m authz.Membership, op *Operation) error {
	var d newWorkflowDraft
	if err := json.Unmarshal(op.Definition, &d); err != nil {
		return fmt.Errorf("read proposed workflow: %w", err)
	}
	var description *string
	if d.Description != "" {
		description = &d.Description
	}
	wf, err := e.Services.Workflows.Create(ctx, workflows.CreateInput{
		Membership: m, Name: d.Name, Description: description, Trigger: d.Trigger, Spec: d.Spec,
	})
	if err != nil {
		return err
	}
	op.Created, op.After = &wf.ID, rawJSON(models.PublicID(models.PrefixWorkflow, wf.ID))
	return nil
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
createSeries creates the draft series Operator suggested.
*/
func (e *Engine) createSeries(ctx context.Context, m authz.Membership, op *Operation) error {
	var d newSeriesDraft
	if err := json.Unmarshal(op.Definition, &d); err != nil {
		return fmt.Errorf("read proposed series: %w", err)
	}
	spec, err := readSeries(d.Spec)
	if err != nil {
		return err
	}
	row, err := e.Services.Series.Create(ctx, series.CreateInput{
		Membership: m, Name: d.Name, Subject: d.Subject, Spec: &spec,
	})
	if err != nil {
		return err
	}
	op.Created, op.After = &row.ID, rawJSON(models.PublicID(models.PrefixSeries, row.ID))
	return nil
}

/*
revertAutomation undoes a workflow or series operation, reporting false
when it moved on since.
*/
func (e *Engine) revertAutomation(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	switch op.Kind {
	case KindWorkflowSet:
		return e.restoreWorkflow(ctx, m, op)
	case KindWorkflowNew:
		return e.dropWorkflow(ctx, m, op)
	case KindSeriesSet:
		return e.restoreSeries(ctx, m, op)
	default:
		return e.dropSeries(ctx, m, op)
	}
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
dropWorkflow archives the workflow Operator drafted, while it never
went live.
*/
func (e *Engine) dropWorkflow(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	if op.Created == nil {
		return false, nil
	}
	wf, err := e.Services.Workflows.Show(ctx, m, *op.Created)
	if err != nil || wf.DeletedAt != nil || wf.LiveVersionID != nil {
		return false, err
	}
	_, err = e.Services.Workflows.Archive(ctx, m, wf.ID)
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

/*
dropSeries deletes the series Operator drafted, while it never went
live.
*/
func (e *Engine) dropSeries(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	if op.Created == nil {
		return false, nil
	}
	err := e.Services.Series.Delete(ctx, m, *op.Created)
	if errors.Is(err, series.ErrNotDraft) {
		return false, nil
	}
	return err == nil, err
}
