/*
|--------------------------------------------------------------------------
| Member Roles
|--------------------------------------------------------------------------
|
| A role is only a key. What a role may do is decided by the permission
| catalog, never by comparing role names, so checks keep working when
| custom roles arrive. Owners hold every workspace permission; there is
| always at least one active owner.
|
*/

package enums

/*
Role is a member's role in a workspace.
*/
type Role string

/*
The roles, from most to least powerful.
*/
const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

/*
Roles returns every role, in the order of the database check.
*/
func Roles() []Role {
	return []Role{RoleOwner, RoleAdmin, RoleMember}
}

/*
Valid reports whether the role is one of the roles.
*/
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember:
		return true
	}

	return false
}
