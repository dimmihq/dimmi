/*
|--------------------------------------------------------------------------
| Create Role
|--------------------------------------------------------------------------
|
| Admins shape custom roles from what an admin or a member may do, so a
| support agent can reply without deleting and a contractor can read
| without changing anything. The base role is chosen once: a role that
| could switch base would silently change what every holder may do.
|
*/

package roles

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateRoleInput is who creates a role, through their membership, and
what it is: its name, an optional description, the built-in role it
narrows and the permissions it keeps.
*/
type CreateRoleInput struct {
	Membership  authz.Membership
	Name        string
	Description *string
	BaseRole    enums.Role
	Permissions []string
}

/*
CreateRole adds a custom role to the caller's workspace.

Rules:
  - The caller needs roles:manage.
  - The role narrows an admin or a member and keeps members:read.
  - Every permission is one a custom role may hold, that the base role
    holds and that the caller holds.
  - The name is unique among the workspace's roles, ignoring case.
  - A workspace has at most 50 custom roles.

Events: [appevents.RoleCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrPermissionRequired],
[exceptions.ErrPermissionNotAssignable],
[exceptions.ErrPermissionAboveBaseRole], [exceptions.ErrPermissionNotHeld],
[exceptions.ErrRoleNameTaken], [exceptions.ErrRoleLimitReached], or a
wrapped database error.
*/
type CreateRole struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle creates the role and returns it, held by nobody yet.
*/
func (a *CreateRole) Handle(ctx context.Context, in CreateRoleInput) (Role, error) {
	if !in.Membership.Can(managePermission) {
		return Role{}, exceptions.ErrForbidden
	}

	permissions := cleanPermissions(in.Permissions)
	if err := checkPermissions(in.Membership, in.BaseRole, permissions); err != nil {
		return Role{}, err
	}

	var role models.Role

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		workspaceID := in.Membership.Workspace.ID

		if _, err := q.LockWorkspace(ctx, workspaceID); err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		count, err := q.CountRoles(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("count roles: %w", err)
		}

		if count >= maxRoles {
			return exceptions.ErrRoleLimitReached.With("limit", maxRoles)
		}

		role, err = q.CreateRole(ctx, models.CreateRoleParams{
			WorkspaceID: workspaceID, Name: str.Clean(in.Name), Description: cleanDescription(in.Description),
			BaseRole: string(in.BaseRole), Permissions: permissions, CreatedByMemberID: &in.Membership.Member.ID,
			Now: a.Clock.Now(),
		})
		if err != nil {
			return nameError(err, "create role")
		}

		return a.Events.Dispatch(ctx, tx, appevents.RoleCreated{
			WorkspaceID: workspaceID, RoleID: role.ID, Name: role.Name, BaseRole: role.BaseRole,
			Permissions: role.Permissions, CreatedBy: in.Membership.Member.ID,
		})
	})
	if err != nil {
		return Role{}, err
	}

	return Role{Role: role}, nil
}

/*
cleanDescription returns the description cleaned, nil when it is empty.
*/
func cleanDescription(description *string) *string {
	if description == nil {
		return nil
	}

	cleaned := str.Clean(*description)
	if cleaned == "" {
		return nil
	}

	return &cleaned
}
