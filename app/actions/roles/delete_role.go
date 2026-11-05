/*
|--------------------------------------------------------------------------
| Delete Role
|--------------------------------------------------------------------------
|
| A custom role goes only once nobody holds it. Falling its holders back
| to the built-in role would quietly give them everything it allows, so
| they are moved to another role first, one at a time or all at once.
|
*/

package roles

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteRoleInput is who deletes a role, through their membership, and
the role.
*/
type DeleteRoleInput struct {
	Membership authz.Membership
	RoleID     ids.UUID
}

/*
DeleteRole deletes a custom role of the caller's workspace.

Rules:
  - The caller needs roles:manage.
  - Roles of other workspaces are not found.
  - A role any member holds is refused, with how many hold it.
  - The caller must hold every permission the role holds.

Events: [appevents.RoleDeleted].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrRoleInUse], [exceptions.ErrPermissionNotHeld], or a
wrapped database error.
*/
type DeleteRole struct {
	DB     *database.DB
	Events *events.Dispatcher
}

/*
Handle deletes the role.
*/
func (a *DeleteRole) Handle(ctx context.Context, in DeleteRoleInput) error {
	if !in.Membership.Can(managePermission) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		workspaceID := in.Membership.Workspace.ID

		if _, err := lockRole(ctx, q, workspaceID, in.RoleID); err != nil {
			return err
		}

		role, err := findRole(ctx, q, workspaceID, in.RoleID)
		if err != nil {
			return err
		}

		if role.MemberCount > 0 {
			return exceptions.ErrRoleInUse.With("member_count", role.MemberCount)
		}

		if bad := outside(role.Role.Permissions, in.Membership.Can); len(bad) > 0 {
			return exceptions.ErrPermissionNotHeld.With("permissions", bad)
		}

		_, err = q.DeleteRole(ctx, models.DeleteRoleParams{WorkspaceID: workspaceID, ID: role.Role.ID})
		if err != nil {
			return fmt.Errorf("delete role: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.RoleDeleted{
			WorkspaceID: workspaceID, RoleID: role.Role.ID, Name: role.Role.Name, DeletedBy: in.Membership.Member.ID,
		})
	})
}
