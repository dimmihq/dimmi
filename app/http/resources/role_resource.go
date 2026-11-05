/*
|--------------------------------------------------------------------------
| Role Resources
|--------------------------------------------------------------------------
|
| Roles as role pickers and the roles page see them. The built-in roles
| are listed beside the workspace's own with no id, since nothing but
| their name identifies them. A permission carries the words that explain
| it and which built-in roles a custom role may narrow to hold it.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/roles"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Role is a built-in or custom role. ID, Description, MemberCount and the
times are null for a built-in role; MemberCount is null too for callers
who do not manage roles.
*/
type Role struct {
	ID          *string    `json:"id"`
	Object      string     `json:"object"`
	Kind        string     `json:"kind"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	BaseRole    string     `json:"base_role"`
	Permissions []string   `json:"permissions"`
	MemberCount *int64     `json:"member_count"`
	CreatedAt   *time.Time `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

/*
RoleSummary names the custom role a member holds. Name is empty until
the caller fills it in from the workspace's roles.
*/
type RoleSummary struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name"`
}

/*
Permission is one permission of the catalog: its key, the ability group
tokens reach it through, what it is called and does, whether a custom
role may hold it and which built-in roles it may then narrow.
*/
type Permission struct {
	Object      string   `json:"object"`
	Key         string   `json:"key"`
	Group       string   `json:"group"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Assignable  bool     `json:"assignable"`
	BaseRoles   []string `json:"base_roles"`
}

/*
NewRole shapes a custom role with the member count to show, nil to
show none.
*/
func NewRole(r roles.Role, memberCount *int64) Role {
	id := models.PublicID(models.PrefixRole, r.Role.ID)
	return Role{
		ID: &id, Object: "role", Kind: "custom", Name: r.Role.Name, Description: r.Role.Description,
		BaseRole: r.Role.BaseRole, Permissions: nonNilStrings(r.Role.Permissions),
		MemberCount: memberCount, CreatedAt: &r.Role.CreatedAt, UpdatedAt: &r.Role.UpdatedAt,
	}
}

/*
NewBuiltInRole shapes a built-in role and what it holds.
*/
func NewBuiltInRole(role enums.Role, name string, permissions []string) Role {
	return Role{
		Object: "role", Kind: "built_in", Name: name, BaseRole: string(role), Permissions: nonNilStrings(permissions),
	}
}

/*
newRoleSummary names the custom role by its id alone, nil for none.
*/
func newRoleSummary(id *ids.UUID) *RoleSummary {
	if id == nil {
		return nil
	}

	return &RoleSummary{ID: models.PublicID(models.PrefixRole, *id), Object: "role"}
}

/*
NameRoles fills in the name of each member's custom role from the
workspace's roles, keyed by public id.
*/
func NameRoles(members []WorkspaceMember, names map[string]string) {
	for i := range members {
		if role := members[i].CustomRole; role != nil {
			role.Name = names[role.ID]
		}
	}
}

/*
nonNilStrings returns the list, empty rather than nil.
*/
func nonNilStrings(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
