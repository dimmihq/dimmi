/*
|--------------------------------------------------------------------------
| Action Definitions
|--------------------------------------------------------------------------
|
| An admin drafts an action, edits its working copy and publishes it as
| an immutable version; the agent runs only published versions. A new
| action starts conservative: anything but a GET writes, and a write
| waits for the customer's confirmation unless the admin says otherwise.
| A disabled action keeps its versions and can be switched back on;
| a deleted one keeps its past calls.
|
*/

package agentactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionresult"
)

/*
maxActions caps the actions one workspace may define.
*/
const maxActions = 500

/*
Definitions manages a workspace's actions.
*/
type Definitions struct {
	DB    *database.DB
	Clock clock.Clock
	Kit   *actionkit.Kit
}

/*
DefinitionInput is an action to create, or the fields of one to change:
nil leaves a field as it is. BodyTemplate set to JSON null clears the
body. Status moves a published action between published and disabled.
*/
type DefinitionInput struct {
	Membership             authz.Membership
	ConnectionID           *ids.UUID
	Name                   *string
	Title                  *string
	Description            *string
	SideEffect             *string
	Confirmation           *string
	Approval               *string
	ApprovalTimeoutMinutes *int32
	Method                 *string
	PathTemplate           *string
	Query                  *map[string]string
	Headers                *map[string]string
	BodyTemplate           *json.RawMessage
	BodyEncoding           *string
	MCPToolName            *string
	BoundArguments         *map[string]any
	InputSchema            *json.RawMessage
	ResponseFields         *[]actionresult.Field
	ErrorMessagePath       *string
	ResponseMaxBytes       *int32
	TimeoutMS              *int32
	Channels               *[]string
	Status                 *string
	Source                 string
}

/*
ListFilter narrows the action list to one status or one connection.
*/
type ListFilter struct {
	Status       *string
	ConnectionID *ids.UUID
}

/*
List returns the workspace's actions by name.
*/
func (d *Definitions) List(ctx context.Context, m authz.Membership, f ListFilter) ([]models.AiAction, error) {
	if err := RequireRead(m); err != nil {
		return nil, err
	}
	var out []models.AiAction
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListActions(ctx, models.ListActionsParams{
			WorkspaceID: m.Workspace.ID, Status: f.Status, ConnectionID: f.ConnectionID,
		})
		return err
	})
	return out, err
}

/*
View is an action with its live version, when it has one.
*/
type View struct {
	Action  models.AiAction
	Version *models.AiActionVersion
}

/*
Show returns one action and its live version.
*/
func (d *Definitions) Show(ctx context.Context, m authz.Membership, id ids.UUID) (View, error) {
	if err := RequireRead(m); err != nil {
		return View{}, err
	}
	var out View
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = loadView(ctx, tx, m.Workspace.ID, id)
		return err
	})
	return out, err
}

/*
Create drafts a new action.
*/
func (d *Definitions) Create(ctx context.Context, in DefinitionInput) (View, error) {
	if err := RequireManage(in.Membership); err != nil {
		return View{}, err
	}
	if in.ConnectionID == nil {
		return View{}, invalidField("connection_required", "connection_id", "name the connection")
	}
	var out View
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		ws := in.Membership.Workspace.ID
		conn, err := findConnection(ctx, tx, ws, *in.ConnectionID)
		if err != nil {
			return invalidField("connection_invalid", "connection_id", "the connection does not exist")
		}
		if err := checkCapacity(ctx, tx, ws); err != nil {
			return err
		}
		s, err := actionkit.Normalize(newSpec(conn, in), conn)
		if err != nil {
			return err
		}
		row, err := models.New(tx).CreateAction(ctx, createParams(ws, s, in, d.Clock))
		if err != nil {
			return actionWriteError(err)
		}
		out = View{Action: row}
		return nil
	})
	return out, err
}

/*
Update edits the working copy, or switches a published action on or
off.
*/
func (d *Definitions) Update(ctx context.Context, id ids.UUID, in DefinitionInput) (View, error) {
	if err := RequireManage(in.Membership); err != nil {
		return View{}, err
	}
	var out View
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		ws := in.Membership.Workspace.ID
		current, err := loadView(ctx, tx, ws, id)
		if err != nil {
			return err
		}
		row, err := d.edit(ctx, tx, current.Action, in)
		if err != nil {
			return err
		}
		if in.Status != nil {
			if row, err = d.setStatus(ctx, tx, row, in); err != nil {
				return err
			}
		}
		out, err = loadView(ctx, tx, ws, row.ID)
		return err
	})
	return out, err
}

/*
edit applies the input's definition fields to the working copy.
*/
func (d *Definitions) edit(ctx context.Context, tx *database.Tx, a models.AiAction, in DefinitionInput,
) (models.AiAction, error) {
	if !in.editsDefinition() {
		return a, nil
	}
	conn, err := findConnection(ctx, tx, a.WorkspaceID, a.ConnectionID)
	if err != nil {
		return a, err
	}
	s, err := actionkit.Normalize(apply(actionkit.FromAction(a), in), conn)
	if err != nil {
		return a, err
	}
	row, err := models.New(tx).UpdateAction(ctx, updateParams(a, s, in, d.Clock))
	if err != nil {
		return a, actionWriteError(err)
	}
	return row, nil
}

/*
setStatus switches a published action off, or back on. A draft is
published through a publication, never by its status.
*/
func (d *Definitions) setStatus(ctx context.Context, tx *database.Tx, a models.AiAction, in DefinitionInput,
) (models.AiAction, error) {
	status := *in.Status
	if status != "published" && status != "disabled" {
		return a, exceptions.ErrActionInvalidTransition.OnField("status")
	}
	if status == "published" && a.PublishedVersionID == nil {
		return a, exceptions.ErrActionInvalidTransition.OnField("status")
	}
	member := in.Membership.Member.ID
	return models.New(tx).SetActionStatus(ctx, models.SetActionStatusParams{
		WorkspaceID: a.WorkspaceID, ID: a.ID, Status: status, MemberID: &member, Now: d.Clock.Now(),
	})
}

/*
Delete removes an action; its past calls keep it.
*/
func (d *Definitions) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := RequireManage(m); err != nil {
		return err
	}
	return d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, err := loadView(ctx, tx, m.Workspace.ID, id); err != nil {
			return err
		}
		member := m.Member.ID
		return models.New(tx).DeleteAction(ctx, models.DeleteActionParams{
			WorkspaceID: m.Workspace.ID, ID: id, MemberID: &member, Now: d.Clock.Now(),
		})
	})
}

/*
Publish snapshots the working copy as the action's next version and
makes it the one the agent runs.
*/
func (d *Definitions) Publish(ctx context.Context, m authz.Membership, id ids.UUID, note string,
) (models.AiActionVersion, error) {
	if err := RequireManage(m); err != nil {
		return models.AiActionVersion{}, err
	}
	var out models.AiActionVersion
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		current, err := loadView(ctx, tx, m.Workspace.ID, id)
		if err != nil {
			return err
		}
		out, err = d.snapshot(ctx, tx, current, publication{membership: m, note: note})
		return err
	})
	return out, err
}

/*
publication is who publishes and the note they leave.
*/
type publication struct {
	membership authz.Membership
	note       string
}

/*
snapshot checks the working copy once more and writes the version.
*/
func (d *Definitions) snapshot(ctx context.Context, tx *database.Tx, v View, p publication,
) (models.AiActionVersion, error) {
	conn, err := findConnection(ctx, tx, v.Action.WorkspaceID, v.Action.ConnectionID)
	if err != nil {
		return models.AiActionVersion{}, err
	}
	s, err := actionkit.Normalize(actionkit.FromAction(v.Action), conn)
	if err != nil {
		return models.AiActionVersion{}, err
	}
	hash := s.Hash()
	if v.Version != nil && v.Version.SpecHash == hash && v.Action.Status == "published" {
		return models.AiActionVersion{}, exceptions.ErrNothingToPublish
	}
	spec, err := json.Marshal(s)
	if err != nil {
		return models.AiActionVersion{}, fmt.Errorf("encode action version: %w", err)
	}
	q, member, now := models.New(tx), p.membership.Member.ID, d.Clock.Now()
	version, err := q.CreateActionVersion(ctx, models.CreateActionVersionParams{
		WorkspaceID: v.Action.WorkspaceID, ActionID: v.Action.ID, Spec: spec, SpecHash: hash,
		Note: nilText(p.note), MemberID: &member, Now: now,
	})
	if err != nil {
		return version, fmt.Errorf("create action version: %w", err)
	}
	_, err = q.PublishAction(ctx, models.PublishActionParams{
		WorkspaceID: v.Action.WorkspaceID, ID: v.Action.ID, VersionID: &version.ID, MemberID: &member, Now: now,
	})
	return version, err
}

/*
Versions lists an action's published versions, newest first.
*/
func (d *Definitions) Versions(ctx context.Context, m authz.Membership, id ids.UUID,
) ([]models.AiActionVersion, error) {
	if err := RequireRead(m); err != nil {
		return nil, err
	}
	var out []models.AiActionVersion
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, err := loadView(ctx, tx, m.Workspace.ID, id); err != nil {
			return err
		}
		var err error
		out, err = models.New(tx).ListActionVersions(ctx, models.ListActionVersionsParams{
			WorkspaceID: m.Workspace.ID, ActionID: id,
		})
		return err
	})
	return out, err
}

/*
loadView finds a live action and its published version, or 404.
*/
func loadView(ctx context.Context, tx *database.Tx, workspaceID, id ids.UUID) (View, error) {
	q := models.New(tx)
	a, err := q.FindAction(ctx, models.FindActionParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return View{}, exceptions.ErrNotFound
	}
	if err != nil {
		return View{}, fmt.Errorf("find action: %w", err)
	}
	out := View{Action: a}
	if a.PublishedVersionID != nil {
		v, err := q.FindActionVersion(ctx, models.FindActionVersionParams{
			WorkspaceID: workspaceID, ID: *a.PublishedVersionID,
		})
		if err == nil {
			out.Version = &v
		}
	}
	return out, nil
}

/*
checkCapacity refuses a workspace that already holds the most actions.
*/
func checkCapacity(ctx context.Context, tx *database.Tx, workspaceID ids.UUID) error {
	n, err := models.New(tx).CountActions(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count actions: %w", err)
	}
	if n >= maxActions {
		return invalidField("too_many_actions", "name", "a workspace holds at most 500 actions")
	}
	return nil
}

/*
actionWriteError maps a duplicate name to a field error.
*/
func actionWriteError(err error) error {
	if database.IsUniqueViolation(err, "") {
		return exceptions.ErrActionNameTaken.OnField("name")
	}
	return fmt.Errorf("save action: %w", err)
}
