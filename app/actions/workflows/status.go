/*
|--------------------------------------------------------------------------
| Workflow Status, Order and History
|--------------------------------------------------------------------------
|
| A live workflow can be paused and resumed on the same version.
| Archiving takes it out of the list and stops it matching at once; a
| reusable workflow another live workflow still calls cannot be
| archived. The list order is the priority when several workflows want
| the same conversation, so reordering is checked against the order the
| editor saw.
|
*/

package workflows

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of changing status and order.
*/
var (
	ErrNotPublished  = fault.New(fault.Conflict, "not_published", "Publish the workflow before making it live.")
	ErrStatusInvalid = fault.New(fault.Invalid, "status_invalid", "Use live or paused.").OnField("status")
	ErrInUse         = fault.New(fault.Conflict, "workflow_in_use",
		"Live workflows still run this workflow. Remove those steps first.")
	ErrOrderStale = fault.New(fault.Conflict, "order_stale",
		"The list changed since you loaded it. Reload and reorder again.").OnField("order")
	ErrNotArchived = fault.New(fault.Conflict, "workflow_not_archived", "This workflow is not archived.")
)

/*
SetStatus pauses a live workflow or makes a paused one live again.
*/
func (w *Workflows) SetStatus(ctx context.Context, m authz.Membership, id ids.UUID, status string,
) (models.Workflow, error) {
	if !m.Can("workflows:manage") {
		return models.Workflow{}, exceptions.ErrForbidden
	}

	if status != "live" && status != "paused" {
		return models.Workflow{}, ErrStatusInvalid
	}

	var out models.Workflow
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := lock(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if wf.LiveVersionID == nil {
			return ErrNotPublished
		}

		out, err = q.SetWorkflowStatus(ctx, models.SetWorkflowStatusParams{
			Status: status, Now: w.Clock.Now(), WorkspaceID: wf.WorkspaceID, ID: wf.ID,
		})
		if err != nil {
			return fmt.Errorf("set workflow status: %w", err)
		}

		return nil
	})
	if err == nil {
		w.changed(ctx, m.Workspace.ID)
	}

	return out, err
}

/*
Archive archives a workflow.
*/
func (w *Workflows) Archive(ctx context.Context, m authz.Membership, id ids.UUID) (models.Workflow, error) {
	if !m.Can("workflows:manage") {
		return models.Workflow{}, exceptions.ErrForbidden
	}

	var out models.Workflow
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := lock(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		callers, err := q.WorkflowsCallingWorkflow(ctx, models.WorkflowsCallingWorkflowParams{
			WorkspaceID: wf.WorkspaceID, WorkflowID: wf.ID,
			PublicID: w.IDs.Encode(string(models.PrefixWorkflow), wf.ID),
		})
		if err != nil {
			return fmt.Errorf("find calling workflows: %w", err)
		}

		if len(callers) > 0 {
			return ErrInUse
		}

		out, err = q.ArchiveWorkflow(ctx, models.ArchiveWorkflowParams{
			Now: w.Clock.Now(), WorkspaceID: wf.WorkspaceID, ID: wf.ID,
		})
		if err != nil {
			return fmt.Errorf("archive workflow: %w", err)
		}

		return nil
	})
	if err == nil {
		w.changed(ctx, m.Workspace.ID)
	}

	return out, err
}

/*
Unarchive brings an archived workflow back at the end of the list. A
workflow that was live comes back paused.
*/
func (w *Workflows) Unarchive(ctx context.Context, m authz.Membership, id ids.UUID) (models.Workflow, error) {
	if !m.Can("workflows:manage") {
		return models.Workflow{}, exceptions.ErrForbidden
	}

	var out models.Workflow
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if wf.DeletedAt == nil {
			return ErrNotArchived
		}

		if err := nameFree(ctx, q, models.WorkflowNameTakenParams{
			WorkspaceID: wf.WorkspaceID, Name: wf.Name, ID: wf.ID,
		}); err != nil {
			return err
		}

		position, err := q.NextWorkflowPosition(ctx, wf.WorkspaceID)
		if err != nil {
			return fmt.Errorf("place workflow: %w", err)
		}

		out, err = q.UnarchiveWorkflow(ctx, models.UnarchiveWorkflowParams{
			Position: position, Now: w.Clock.Now(), WorkspaceID: wf.WorkspaceID, ID: wf.ID,
		})
		if err != nil {
			return fmt.Errorf("unarchive workflow: %w", err)
		}

		return nil
	})

	return out, err
}

/*
Reorder puts the workflows in the given order. The order must list
every workflow that is not archived, exactly once.
*/
func (w *Workflows) Reorder(ctx context.Context, m authz.Membership, order []ids.UUID) ([]models.Workflow, error) {
	if !m.Can("workflows:manage") {
		return nil, exceptions.ErrForbidden
	}

	ws := m.Workspace.ID
	var out []models.Workflow

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := q.ListWorkflows(ctx, models.ListWorkflowsParams{WorkspaceID: ws})
		if err != nil {
			return fmt.Errorf("list workflows: %w", err)
		}

		if !sameSet(current, order) {
			return ErrOrderStale
		}

		now := w.Clock.Now()
		for i, id := range order {
			err := q.SetWorkflowPosition(ctx, models.SetWorkflowPositionParams{
				Position: int32((i + 1) * 1024), Now: now, WorkspaceID: ws, ID: id,
			})
			if err != nil {
				return fmt.Errorf("move workflow: %w", err)
			}
		}

		out, err = q.ListWorkflows(ctx, models.ListWorkflowsParams{WorkspaceID: ws})

		return err
	})
	if err == nil {
		w.changed(ctx, ws)
	}

	return out, err
}

/*
sameSet reports whether the order lists exactly the workflows.
*/
func sameSet(current []models.Workflow, order []ids.UUID) bool {
	if len(current) != len(order) {
		return false
	}

	for _, wf := range current {
		if !slices.Contains(order, wf.ID) {
			return false
		}
	}

	return true
}
