/*
|--------------------------------------------------------------------------
| Procedures
|--------------------------------------------------------------------------
|
| How the team wants dimmi to handle one kind of request, kept as a
| working copy anyone allowed edits and a live copy dimmi reads. Saving
| names the lock version the editor started from, so two editors never
| overwrite each other. Publishing checks the working copy whole,
| including that every action it takes exists and is published, so
| dimmi never follows a step it cannot carry out. Reading takes
| ai_actions:read; changing takes ai_agent:manage.
|
*/

package procedures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
The failures of procedures.
*/
var (
	ErrNotFound  = fault.New(fault.NotFound, "procedure_not_found", "No procedure of this workspace has this id.")
	ErrNameTaken = fault.New(fault.Conflict, "name_taken", "Another procedure already has this name.").
			OnField("name")
	ErrNameInvalid  = fault.New(fault.Invalid, "name_invalid", "Names are 1 to 80 characters.").OnField("name")
	ErrEditConflict = fault.New(fault.Conflict, "edit_conflict",
		"Someone else saved this procedure since you opened it. Reload to see their changes.")
	ErrInvalid = fault.New(fault.Invalid, "procedure_invalid", "The procedure cannot be followed as it is.")
)

/*
Procedures manages a workspace's procedures.
*/
type Procedures struct {
	DB    *database.DB
	Clock clock.Clock
	IDs   *ids.Registry
}

/*
Listed is a procedure with how many conversations dimmi followed it in
over the last 30 days.
*/
type Listed struct {
	models.Procedure
	Uses int32
}

/*
SaveInput is a save of a procedure's working copy. Nil fields stay as
they are; LockVersion is the version the editor started from.
*/
type SaveInput struct {
	Membership  authz.Membership
	ID          ids.UUID
	LockVersion int32
	Name        *string
	Definition  json.RawMessage
}

/*
List returns the workspace's procedures by name, with their use.
*/
func (p *Procedures) List(ctx context.Context, m authz.Membership) ([]Listed, error) {
	if !m.Can("ai_actions:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []Listed
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		rows, err := q.ListProcedures(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list procedures: %w", err)
		}

		uses, err := q.ProcedureUses(ctx, models.ProcedureUsesParams{
			WorkspaceID: m.Workspace.ID, Since: p.Clock.Now().Add(-30 * 24 * time.Hour),
		})
		if err != nil {
			return fmt.Errorf("count procedure uses: %w", err)
		}

		counts := map[ids.UUID]int32{}
		for _, u := range uses {
			counts[u.ID] = u.Conversations
		}

		for _, r := range rows {
			out = append(out, Listed{Procedure: r, Uses: counts[r.ID]})
		}

		return nil
	})

	return out, err
}

/*
Show returns one procedure.
*/
func (p *Procedures) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.Procedure, error) {
	if !m.Can("ai_actions:read") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	var out models.Procedure
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = find(ctx, models.New(tx), m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
Create stores a new, empty draft procedure.
*/
func (p *Procedures) Create(ctx context.Context, m authz.Membership, name string) (models.Procedure, error) {
	if !m.Can("ai_agent:manage") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	name, err := cleanName(name)
	if err != nil {
		return models.Procedure{}, err
	}

	draft, _ := json.Marshal(procedure.Empty())
	ws, now := m.Workspace.ID, p.Clock.Now()
	var out models.Procedure
	err = p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		free := models.ProcedureNameTakenParams{WorkspaceID: ws, Name: name, ID: ids.Nil()}
		if err := nameFree(ctx, q, free); err != nil {
			return err
		}

		out, err = q.CreateProcedure(ctx, models.CreateProcedureParams{
			ID: ids.NewV7At(now), WorkspaceID: ws, Name: name, Draft: draft, MemberID: &m.Member.ID, Now: now,
		})

		return wrap("create procedure", err)
	})

	return out, err
}

/*
Save saves the working copy.
*/
func (p *Procedures) Save(ctx context.Context, in SaveInput) (models.Procedure, error) {
	if !in.Membership.Can("ai_agent:manage") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID
	var out models.Procedure
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}

		params, err := saveParams(ctx, q, current, in)
		if err != nil {
			return err
		}

		params.Now = p.Clock.Now()
		out, err = q.SaveProcedureDraft(ctx, params)
		if errors.Is(err, database.ErrNotFound) {
			return ErrEditConflict
		}

		return wrap("save procedure", err)
	})

	return out, err
}

/*
saveParams is the saved row: the given name and definition, checked
for shape, over the current ones.
*/
func saveParams(ctx context.Context, q *models.Queries, current models.Procedure, in SaveInput,
) (models.SaveProcedureDraftParams, error) {
	p := models.SaveProcedureDraftParams{
		Name: current.Name, Draft: current.Draft, MemberID: &in.Membership.Member.ID,
		WorkspaceID: current.WorkspaceID, ID: current.ID, LockVersion: in.LockVersion,
	}
	if in.LockVersion == 0 {
		p.LockVersion = current.LockVersion
	}

	if in.Name != nil {
		name, err := cleanName(*in.Name)
		if err != nil {
			return p, err
		}

		free := models.ProcedureNameTakenParams{WorkspaceID: current.WorkspaceID, Name: name, ID: current.ID}
		if err := nameFree(ctx, q, free); err != nil {
			return p, err
		}

		p.Name = name
	}

	if len(in.Definition) > 0 {
		d, err := procedure.Parse(in.Definition)
		if err != nil {
			return p, ErrInvalid.With("reason", err.Error())
		}

		p.Draft, _ = json.Marshal(d)
	}

	return p, nil
}

/*
find returns one procedure of the workspace.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Procedure, error) {
	found, err := q.FindProcedure(ctx, models.FindProcedureParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return found, ErrNotFound
	}

	return found, wrap("find procedure", err)
}

/*
nameFree refuses a name another procedure has.
*/
func nameFree(ctx context.Context, q *models.Queries, p models.ProcedureNameTakenParams) error {
	taken, err := q.ProcedureNameTaken(ctx, p)
	if err != nil {
		return fmt.Errorf("check procedure name: %w", err)
	}

	if taken {
		return ErrNameTaken
	}

	return nil
}

/*
cleanName trims a name and checks its length.
*/
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 80 {
		return "", ErrNameInvalid
	}

	return name, nil
}

/*
wrap names what failed, leaving no error alone.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}
