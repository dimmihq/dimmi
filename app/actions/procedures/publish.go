/*
|--------------------------------------------------------------------------
| Publishing Procedures
|--------------------------------------------------------------------------
|
| Publishing makes the working copy what dimmi follows. A procedure is
| published only whole: when it applies, at least one step, every step
| written, and every action step naming a workspace action that exists
| and is published. Unpublishing takes it out of dimmi's hands at once
| and keeps the working copy.
|
*/

package procedures

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
Publish checks the working copy and makes it live.
*/
func (p *Procedures) Publish(ctx context.Context, m authz.Membership, id ids.UUID) (models.Procedure, error) {
	if !m.Can("ai_agent:manage") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	var out models.Procedure
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if err := p.publishable(ctx, q, current); err != nil {
			return err
		}

		out, err = q.PublishProcedure(ctx, models.PublishProcedureParams{
			MemberID: &m.Member.ID, Now: new(p.Clock.Now()), WorkspaceID: m.Workspace.ID, ID: id,
		})

		return wrap("publish procedure", err)
	})

	return out, err
}

/*
publishable lists every problem of the working copy, actions included.
*/
func (p *Procedures) publishable(ctx context.Context, q *models.Queries, current models.Procedure) error {
	d, err := procedure.Parse(current.Draft)
	if err != nil {
		return ErrInvalid.With("reason", err.Error())
	}

	issues := procedure.Check(d)
	for _, public := range procedure.ActionIDs(d) {
		ok, err := p.actionReady(ctx, q, current.WorkspaceID, public)
		if err != nil {
			return err
		}

		if !ok {
			issues = append(issues, procedure.Issue{
				Code: "reference_missing", Field: "action_id", StepID: stepWith(d, public),
				Message: "An action this procedure takes is not published.",
			})
		}
	}

	if len(issues) > 0 {
		return ErrInvalid.With("errors", issues)
	}

	return nil
}

/*
actionReady reports whether the action exists and is published.
*/
func (p *Procedures) actionReady(ctx context.Context, q *models.Queries, ws ids.UUID, public string) (bool, error) {
	id, ok := p.actionID(public)
	if !ok {
		return false, nil
	}

	action, err := q.FindAction(ctx, models.FindActionParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find action: %w", err)
	}

	return action.Status == "published", nil
}

/*
actionID decodes an action's public id, false when it is not one.
*/
func (p *Procedures) actionID(public string) (ids.UUID, bool) {
	id, err := p.IDs.Decode(string(models.PrefixAiAction), public)

	return id, err == nil
}

/*
stepWith returns the first step that takes the action.
*/
func stepWith(d procedure.Definition, action string) string {
	for _, s := range d.Steps {
		if s.ActionID == action {
			return s.ID
		}
	}

	return ""
}

/*
Unpublish takes a procedure out of dimmi's hands.
*/
func (p *Procedures) Unpublish(ctx context.Context, m authz.Membership, id ids.UUID) (models.Procedure, error) {
	if !m.Can("ai_agent:manage") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	var out models.Procedure
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		var err error
		out, err = q.UnpublishProcedure(ctx, models.UnpublishProcedureParams{
			MemberID: &m.Member.ID, Now: p.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})

		return wrap("unpublish procedure", err)
	})

	return out, err
}

/*
Delete removes a procedure.
*/
func (p *Procedures) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("ai_agent:manage") {
		return exceptions.ErrForbidden
	}

	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		return wrap("delete procedure", q.DeleteProcedure(ctx, models.DeleteProcedureParams{
			WorkspaceID: m.Workspace.ID, ID: id, Now: new(p.Clock.Now()),
		}))
	})
}
