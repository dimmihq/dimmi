/*
|--------------------------------------------------------------------------
| List Roles
|--------------------------------------------------------------------------
|
| Role pickers show the built-in roles beside the workspace's own, so
| anyone who may see the members can read them all. How many members
| hold each custom role is shown only to those who manage roles.
|
*/

package roles

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
Roles is the workspace's custom roles by name. Counted says whether each
carries how many members hold it.
*/
type Roles struct {
	Custom  []Role
	Counted bool
}

/*
ListRoles returns the custom roles of the caller's workspace.

Rules:
  - The caller needs members:read.
  - Member counts are kept for callers with roles:manage.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListRoles struct {
	DB *database.DB
}

/*
Handle returns the roles.
*/
func (a *ListRoles) Handle(ctx context.Context, m authz.Membership) (Roles, error) {
	if !m.Can(readPermission) {
		return Roles{}, exceptions.ErrForbidden
	}

	var rows []models.ListRolesRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListRoles(ctx, m.Workspace.ID)

		return err
	})
	if err != nil {
		return Roles{}, fmt.Errorf("list roles: %w", err)
	}

	counted := m.Can(managePermission)
	out := Roles{Custom: make([]Role, 0, len(rows)), Counted: counted}

	for _, row := range rows {
		role := Role{Role: row.Role}
		if counted {
			role.MemberCount = row.MemberCount
		}

		out.Custom = append(out.Custom, role)
	}

	return out, nil
}

/*
ShowRoleInput is who reads a role, through their membership, and the
role.
*/
type ShowRoleInput struct {
	Membership authz.Membership
	RoleID     ids.UUID
}

/*
ShowRole returns a custom role of the caller's workspace.

Rules:
  - The caller needs members:read.
  - Roles of other workspaces are not found.
  - The member count is kept for callers with roles:manage.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowRole struct {
	DB *database.DB
}

/*
Handle returns the role.
*/
func (a *ShowRole) Handle(ctx context.Context, in ShowRoleInput) (Role, error) {
	if !in.Membership.Can(readPermission) {
		return Role{}, exceptions.ErrForbidden
	}

	var role Role

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		role, err = findRole(ctx, models.New(tx), in.Membership.Workspace.ID, in.RoleID)

		return err
	})
	if err != nil {
		return Role{}, err
	}

	if !in.Membership.Can(managePermission) {
		role.MemberCount = 0
	}

	return role, nil
}
