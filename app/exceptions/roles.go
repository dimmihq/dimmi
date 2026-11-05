/*
|--------------------------------------------------------------------------
| Role Errors
|--------------------------------------------------------------------------
|
| What can go wrong while shaping custom roles. A custom role only ever
| narrows a built-in one, and nobody hands out power they do not hold
| themselves, so every widening is refused with the permissions at fault.
| A role someone holds is never deleted underneath them: falling back to
| the built-in role would quietly give them more.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Refusals of a role's contents.

ErrRoleNameTaken refuses a name another custom role of the workspace
uses. ErrPermissionNotAssignable refuses a permission no custom role may
list, ErrPermissionAboveBaseRole one its built-in role does not hold, and
ErrPermissionNotHeld one the caller does not hold. ErrPermissionRequired
refuses a role without the permission every member keeps. Each names the
permissions at fault.
*/
var (
	ErrRoleNameTaken = fault.New(fault.Invalid, "role_name_taken",
		"Another role in this workspace already uses this name.").OnField("name")
	ErrPermissionNotAssignable = fault.New(fault.Invalid, "permission_not_assignable",
		"A custom role cannot hold one of these permissions.").OnField("permissions")
	ErrPermissionAboveBaseRole = fault.New(fault.Invalid, "permission_above_base_role",
		"A custom role can only hold what its base role holds.").OnField("permissions")
	ErrPermissionNotHeld = fault.New(fault.Invalid, "permission_not_held",
		"You can only hand out permissions you hold yourself.").OnField("permissions")
	ErrPermissionRequired = fault.New(fault.Invalid, "permission_required",
		"Every role lets its members see the other members.").OnField("permissions")
)

/*
ErrRoleInUse refuses to delete a role members still hold, with how many.
ErrRoleLimitReached refuses a role past the workspace's limit.
*/
var (
	ErrRoleInUse = fault.New(fault.Conflict, "role_in_use",
		"Members still hold this role. Move them to another role first.")
	ErrRoleLimitReached = fault.New(fault.RateLimited, "entitlement_limit_reached",
		"This workspace has as many custom roles as it may have.")
)
