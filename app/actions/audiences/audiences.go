/*
|--------------------------------------------------------------------------
| Saved Audiences
|--------------------------------------------------------------------------
|
| A saved audience is a rule with a name, written once and used wherever
| the product asks "who". Saving checks the rule whole: every field and
| operator, every saved audience it names exists, and nesting stays two
| levels deep without loops, so a rule in use never stops making sense.
| Saving names the lock version the editor started from, so two editors
| never overwrite each other. An audience a live workflow runs on cannot
| be deleted. Reading takes audiences:read; changing takes
| audiences:manage.
|
*/

package audiences

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The failures of saved audiences.
*/
var (
	ErrNotFound  = fault.New(fault.NotFound, "audience_not_found", "No audience of this workspace has this id.")
	ErrNameTaken = fault.New(fault.Conflict, "name_taken", "Another audience already has this name.").
			OnField("name")
	ErrNameInvalid = fault.New(fault.Invalid, "name_invalid", "Names are 1 to 80 characters.").OnField("name")
	ErrDescription = fault.New(fault.Invalid, "description_invalid", "Descriptions are 500 characters at most.").
			OnField("description")
	ErrEditConflict = fault.New(fault.Conflict, "edit_conflict",
		"Someone else saved this audience since you opened it. Reload to see their changes.")
	ErrInvalid = fault.New(fault.Invalid, "audience_invalid", "The rule cannot be used as it is.")
	ErrInUse   = fault.New(fault.Conflict, "audience_in_use",
		"Live workflows or other audiences use this audience. Remove it from them first.")
)

/*
Audiences manages a workspace's saved audiences.
*/
type Audiences struct {
	DB    *database.DB
	Clock clock.Clock
	IDs   *ids.Registry
}

/*
SaveInput creates or changes a saved audience. On a change, nil fields
stay as they are and LockVersion is the version the editor started
from; zero skips the check.
*/
type SaveInput struct {
	Membership  authz.Membership
	ID          ids.UUID
	LockVersion int32
	Name        *string
	Description *string
	Rules       json.RawMessage
}

/*
User is something that names a saved audience. Live is set for a
workflow whose live version runs on it.
*/
type User = models.AudienceUsersRow

/*
List returns the workspace's saved audiences by name.
*/
func (a *Audiences) List(ctx context.Context, m authz.Membership) ([]models.Audience, error) {
	if !m.Can("audiences:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.Audience
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListAudiences(ctx, m.Workspace.ID)

		return wrap("list audiences", err)
	})

	return out, err
}

/*
Show returns one saved audience with what uses it.
*/
func (a *Audiences) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.Audience, []User, error) {
	if !m.Can("audiences:read") {
		return models.Audience{}, nil, exceptions.ErrForbidden
	}

	var out models.Audience
	var users []User
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		if out, err = find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		users, err = usersOf(ctx, q, out)

		return err
	})

	return out, users, err
}

/*
Create stores a new saved audience.
*/
func (a *Audiences) Create(ctx context.Context, in SaveInput) (models.Audience, error) {
	m := in.Membership
	if !m.Can("audiences:manage") {
		return models.Audience{}, exceptions.ErrForbidden
	}

	now := a.Clock.Now()
	in.ID = ids.NewV7At(now)
	var out models.Audience
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := a.saved(ctx, q, models.Audience{ID: in.ID, WorkspaceID: m.Workspace.ID}, in)
		if err != nil {
			return err
		}

		out, err = q.CreateAudience(ctx, models.CreateAudienceParams{
			ID: row.ID, WorkspaceID: row.WorkspaceID, Name: row.Name, Description: row.Description,
			Rules: row.Rules, MemberID: &m.Member.ID, Now: now,
		})

		return wrap("create audience", err)
	})

	return out, err
}

/*
Save changes a saved audience.
*/
func (a *Audiences) Save(ctx context.Context, in SaveInput) (models.Audience, error) {
	if !in.Membership.Can("audiences:manage") {
		return models.Audience{}, exceptions.ErrForbidden
	}

	var out models.Audience
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, in.Membership.Workspace.ID, in.ID)
		if err != nil {
			return err
		}

		row, err := a.saved(ctx, q, current, in)
		if err != nil {
			return err
		}

		lock := in.LockVersion
		if lock == 0 {
			lock = current.LockVersion
		}

		out, err = q.SaveAudience(ctx, models.SaveAudienceParams{
			Name: row.Name, Description: row.Description, Rules: row.Rules, MemberID: &in.Membership.Member.ID,
			Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID, LockVersion: lock,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrEditConflict
		}

		return wrap("save audience", err)
	})

	return out, err
}

/*
Delete removes a saved audience nothing live uses.
*/
func (a *Audiences) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("audiences:manage") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		users, err := usersOf(ctx, q, current)
		if err != nil {
			return err
		}

		if blocking := blockers(users); len(blocking) > 0 {
			return ErrInUse.With("used_by", blocking)
		}

		_, err = q.DeleteAudience(ctx, models.DeleteAudienceParams{
			Now: new(a.Clock.Now()), MemberID: &m.Member.ID, WorkspaceID: m.Workspace.ID, ID: id,
		})

		return wrap("delete audience", err)
	})
}

/*
saved applies the input over the current row and checks the result.
*/
func (a *Audiences) saved(ctx context.Context, q *models.Queries, row models.Audience, in SaveInput,
) (models.Audience, error) {
	if in.Name != nil {
		name, err := cleanName(*in.Name)
		if err != nil {
			return row, err
		}

		row.Name = name
	}

	if in.Description != nil {
		row.Description = cleanDescription(*in.Description)
		if row.Description != nil && utf8.RuneCountInString(*row.Description) > 500 {
			return row, ErrDescription
		}
	}

	if row.Name == "" {
		return row, ErrNameInvalid
	}

	if err := nameFree(ctx, q, row); err != nil {
		return row, err
	}

	if len(in.Rules) > 0 || row.Rules == nil {
		rules, err := a.check(ctx, q, row, in.Rules)
		if err != nil {
			return row, err
		}

		row.Rules = rules
	}

	return row, nil
}

/*
usersOf lists what names a saved audience.
*/
func usersOf(ctx context.Context, q *models.Queries, row models.Audience) ([]User, error) {
	users, err := q.AudienceUsers(ctx, models.AudienceUsersParams{
		WorkspaceID: row.WorkspaceID, AudienceID: row.ID,
		PublicID: models.PublicID(models.PrefixAudience, row.ID),
	})

	return users, wrap("list audience users", err)
}

/*
blockers are the users that keep a saved audience from being deleted:
other saved audiences, and live workflows.
*/
func blockers(users []User) []string {
	var out []string
	for _, u := range users {
		if u.Kind == "audience" || u.Live {
			out = append(out, u.Name)
		}
	}

	return out
}

/*
find returns one saved audience of the workspace.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Audience, error) {
	found, err := q.FindAudience(ctx, models.FindAudienceParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return found, ErrNotFound
	}

	return found, wrap("find audience", err)
}

/*
nameFree refuses a name another saved audience has.
*/
func nameFree(ctx context.Context, q *models.Queries, row models.Audience) error {
	taken, err := q.AudienceNameTaken(ctx, models.AudienceNameTakenParams{
		WorkspaceID: row.WorkspaceID, Name: row.Name, ID: row.ID,
	})
	if err != nil {
		return fmt.Errorf("check audience name: %w", err)
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
cleanDescription trims a description, nil when empty.
*/
func cleanDescription(text string) *string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	return &text
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
