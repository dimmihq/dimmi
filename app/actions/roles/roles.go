/*
|--------------------------------------------------------------------------
| Role Rules
|--------------------------------------------------------------------------
|
| What every role action shares. A custom role is a named part of an
| admin's or a member's permissions: it only ever narrows the built-in
| role it is based on, keeps the permission every member needs to see the
| people they work with, and never holds what only owners may do. Nobody
| hands out a permission they do not hold themselves, so a member allowed
| to manage roles still cannot make a role stronger than their own.
|
*/

package roles

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Permissions the role actions check: anyone who may see the members sees
the roles they hold, and changing roles takes roles:manage.
*/
const (
	readPermission   = "members:read"
	managePermission = "roles:manage"
)

/*
alwaysHeld is the permission every role keeps.
*/
const alwaysHeld = "members:read"

/*
maxRoles is how many custom roles a workspace may have.
*/
const maxRoles = 50

/*
nameConstraint is the unique index that keeps role names apart.
*/
const nameConstraint = "roles_workspace_id_name_key"

/*
Role is a custom role with how many members hold it.
*/
type Role struct {
	Role        models.Role
	MemberCount int64
}

/*
checkPermissions refuses a list a custom role of the base role may not
hold, or that the caller could not hand out, naming the permissions at
fault.
*/
func checkPermissions(m authz.Membership, base enums.Role, permissions []string) error {
	if !slices.Contains(appauth.CustomBaseRoles(), base) {
		return exceptions.ErrInvalidTransition.OnField("base_role").With("to", string(base))
	}

	if !slices.Contains(permissions, alwaysHeld) {
		return exceptions.ErrPermissionRequired.With("permissions", []string{alwaysHeld})
	}

	if bad := outside(permissions, appauth.Assignable); len(bad) > 0 {
		return exceptions.ErrPermissionNotAssignable.With("permissions", bad)
	}

	held := appauth.RolePermissions(base)
	if bad := outside(permissions, func(p string) bool { return slices.Contains(held, p) }); len(bad) > 0 {
		return exceptions.ErrPermissionAboveBaseRole.With("permissions", bad)
	}

	if bad := outside(permissions, m.Can); len(bad) > 0 {
		return exceptions.ErrPermissionNotHeld.With("permissions", bad)
	}

	return nil
}

/*
outside returns the permissions the test refuses.
*/
func outside(permissions []string, allowed func(string) bool) []string {
	var bad []string

	for _, permission := range permissions {
		if !allowed(permission) {
			bad = append(bad, permission)
		}
	}

	return bad
}

/*
cleanPermissions returns the permissions sorted and without repeats.
*/
func cleanPermissions(permissions []string) []string {
	out := slices.Clone(permissions)
	slices.Sort(out)

	return slices.Compact(out)
}

/*
lockRole returns the custom role and holds its row until the transaction
ends, or exceptions.ErrNotFound.
*/
func lockRole(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.Role, error) {
	role, err := q.LockRole(ctx, models.LockRoleParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return role, exceptions.ErrNotFound
	}

	if err != nil {
		return role, fmt.Errorf("lock role: %w", err)
	}

	return role, nil
}

/*
findRole returns the custom role with its member count, or
exceptions.ErrNotFound.
*/
func findRole(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (Role, error) {
	row, err := q.FindRole(ctx, models.FindRoleParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return Role{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Role{}, fmt.Errorf("find role: %w", err)
	}

	return Role{Role: row.Role, MemberCount: row.MemberCount}, nil
}

/*
nameError turns a clash with another role's name into its refusal.
*/
func nameError(err error, attempt string) error {
	if database.IsUniqueViolation(err, nameConstraint) {
		return exceptions.ErrRoleNameTaken
	}

	return fmt.Errorf("%s: %w", attempt, err)
}
