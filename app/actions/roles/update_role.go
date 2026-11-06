/*
|--------------------------------------------------------------------------
| Update Role
|--------------------------------------------------------------------------
|
| Editing a custom role changes what every holder may do on their very
| next request. Its base role never changes, and the same limits apply
| as when it was made: an editor cannot raise it past its base role or
| past what they hold themselves.
|
*/

package roles

import (
	"context"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateRoleInput is who edits a role, through their membership, the role
and what changes: a nil field keeps its value, and a description of ""
clears it.
*/
type UpdateRoleInput struct {
	Membership  authz.Membership
	RoleID      ids.UUID
	Name        *string
	Description *string
	Permissions *[]string
}

/*
UpdateRole edits a custom role of the caller's workspace.

Rules:
  - The caller needs roles:manage.
  - Roles of other workspaces are not found.
  - The permissions follow the same rules as when the role was made.
  - The caller must hold every permission the role holds, so nobody
    edits a role stronger than their own.
  - Values equal to the current ones change nothing.

Events: [appevents.RoleUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrPermissionRequired], [exceptions.ErrPermissionNotAssignable],
[exceptions.ErrPermissionAboveBaseRole], [exceptions.ErrPermissionNotHeld],
[exceptions.ErrRoleNameTaken], or a wrapped database error.
*/
type UpdateRole struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle edits the role and returns it as it now is.
*/
func (a *UpdateRole) Handle(ctx context.Context, in UpdateRoleInput) (Role, error) {
	if !in.Membership.Can(managePermission) {
		return Role{}, exceptions.ErrForbidden
	}

	var out Role

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		workspaceID := in.Membership.Workspace.ID

		role, err := lockRole(ctx, q, workspaceID, in.RoleID)
		if err != nil {
			return err
		}

		if err := a.edit(ctx, tx, role, in); err != nil {
			return err
		}

		out, err = findRole(ctx, q, workspaceID, role.ID)

		return err
	})

	return out, err
}

/*
edit checks the change against the role and the caller, writes it and
announces what changed.
*/
func (a *UpdateRole) edit(ctx context.Context, tx *database.Tx, role models.Role, in UpdateRoleInput) error {
	if bad := outside(role.Permissions, in.Membership.Can); len(bad) > 0 {
		return exceptions.ErrPermissionNotHeld.With("permissions", bad)
	}

	next := models.UpdateRoleParams{
		WorkspaceID: role.WorkspaceID, ID: role.ID, Name: role.Name, Description: role.Description,
		Permissions: role.Permissions, Now: a.Clock.Now(),
	}

	if in.Name != nil {
		next.Name = str.Clean(*in.Name)
	}

	if in.Description != nil {
		next.Description = cleanDescription(in.Description)
	}

	if in.Permissions != nil {
		next.Permissions = cleanPermissions(*in.Permissions)
		if err := checkPermissions(in.Membership, enums.Role(role.BaseRole), next.Permissions); err != nil {
			return err
		}
	}

	changes := roleChanges(role, next)
	if len(changes) == 0 {
		return nil
	}

	if _, err := models.New(tx).UpdateRole(ctx, next); err != nil {
		return nameError(err, "update role")
	}

	return a.Events.Dispatch(ctx, tx, appevents.RoleUpdated{
		WorkspaceID: role.WorkspaceID, RoleID: role.ID, Name: next.Name, Changes: changes,
		UpdatedBy: in.Membership.Member.ID,
	})
}

/*
roleChanges returns, per field that changed, what it was and what it
became; for the permissions, which were added and which removed.
*/
func roleChanges(role models.Role, next models.UpdateRoleParams) map[string]any {
	changes := map[string]any{}

	if next.Name != role.Name {
		changes["name"] = map[string]any{"from": role.Name, "to": next.Name}
	}

	if text(next.Description) != text(role.Description) {
		changes["description"] = map[string]any{"from": role.Description, "to": next.Description}
	}

	added := outside(next.Permissions, func(p string) bool { return slices.Contains(role.Permissions, p) })
	removed := outside(role.Permissions, func(p string) bool { return slices.Contains(next.Permissions, p) })

	if len(added) > 0 || len(removed) > 0 {
		changes["permissions"] = map[string]any{"added": nonNil(added), "removed": nonNil(removed)}
	}

	return changes
}

/*
text returns the text, "" for none.
*/
func text(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

/*
nonNil returns the list, empty rather than nil, so it records as [].
*/
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
