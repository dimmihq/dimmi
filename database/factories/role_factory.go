/*
|--------------------------------------------------------------------------
| Role Factory
|--------------------------------------------------------------------------
|
| Custom roles for tests, written with the same queries the product uses,
| and a member given one the way an assignment gives it: their built-in
| role becomes the custom role's base role.
|
*/

package factories

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RoleOf is a custom role to create: its workspace, its name, the built-in
role it narrows and what it keeps.
*/
type RoleOf struct {
	Workspace   models.Workspace
	Name        string
	BaseRole    enums.Role
	Permissions []string
}

/*
Role creates the custom role.
*/
func Role(ctx context.Context, a *app.App, r RoleOf) (models.Role, error) {
	var role models.Role

	scoped := tenancy.With(ctx, r.Workspace.ID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error

		role, err = models.New(tx).CreateRole(ctx, models.CreateRoleParams{
			WorkspaceID: r.Workspace.ID, Name: r.Name, BaseRole: string(r.BaseRole), Permissions: r.Permissions,
			Now: a.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return role, fmt.Errorf("create role: %w", err)
	}

	return role, nil
}

/*
WithCustomRole gives the member the custom role and its base role.
*/
func WithCustomRole(
	ctx context.Context, a *app.App, m models.WorkspaceMember, r models.Role,
) (models.WorkspaceMember, error) {
	var member models.WorkspaceMember

	scoped := tenancy.With(ctx, m.WorkspaceID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error

		member, err = models.New(tx).SetMemberCustomRole(ctx, models.SetMemberCustomRoleParams{
			WorkspaceID: m.WorkspaceID, ID: m.ID, Role: r.BaseRole, CustomRoleID: &r.ID, Now: a.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return member, fmt.Errorf("give custom role: %w", err)
	}

	return member, nil
}
