/*
|--------------------------------------------------------------------------
| List Permissions
|--------------------------------------------------------------------------
|
| The role editor shows every permission with whether a custom role may
| hold it and which built-in roles it may then narrow, so the page never
| offers a box the server would refuse.
|
*/

package roles

import (
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
PermissionInfo is one permission of the catalog: its key, its ability
group, whether a custom role may hold it, and the built-in roles a
custom role may then narrow.
*/
type PermissionInfo struct {
	Key        string
	Group      string
	Assignable bool
	BaseRoles  []enums.Role
}

/*
ListPermissions returns the permission catalog, for the role editor.

Rules:
  - The caller needs members:read.

Errors: [exceptions.ErrForbidden].
*/
type ListPermissions struct {
	Catalog *appauth.Catalog
}

/*
Handle returns every permission, sorted by key.
*/
func (a *ListPermissions) Handle(m authz.Membership) ([]PermissionInfo, error) {
	if !m.Can(readPermission) {
		return nil, exceptions.ErrForbidden
	}

	keys := a.Catalog.Permissions()
	out := make([]PermissionInfo, 0, len(keys))

	for _, key := range keys {
		group, _ := a.Catalog.Group(key)
		out = append(out, PermissionInfo{
			Key: key, Group: group, Assignable: appauth.Assignable(key), BaseRoles: appauth.BaseRoles(key),
		})
	}

	return out, nil
}
