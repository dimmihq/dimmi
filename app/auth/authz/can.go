/*
|--------------------------------------------------------------------------
| Authorization
|--------------------------------------------------------------------------
|
| A caller's power in a workspace is computed on every request, never
| stored: the permissions of their role, and for a token only those
| whose ability group the token holds and that the role it was minted
| under held. Demoting a member therefore shrinks every token they
| minted at once, a token can never do more than its member, and a
| promotion never widens a token minted before it.
|
*/

package authz

import (
	"slices"

	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
Principal is a caller acting in one workspace with the permissions
worked out for them there.
*/
type Principal struct {
	Role        enums.Role
	Permissions []string
}

/*
Can reports whether the principal holds the permission. An unknown
permission is never held.
*/
func Can(p Principal, permission string) bool {
	return slices.Contains(p.Permissions, permission)
}

/*
EffectivePermissions returns what a member of the role may do through a
credential with the abilities, sorted. Sessions and the user's own apps
carry every ability, so they get the role's permissions as they are.
*/
func EffectivePermissions(catalog *auth.Catalog, role enums.Role, abilities []string) []string {
	groups := auth.ExpandAbilities(abilities)

	return slices.DeleteFunc(auth.RolePermissions(role), func(permission string) bool {
		group, ok := catalog.Group(permission)

		return !ok || !slices.Contains(groups, group)
	})
}

/*
PrincipalPermissions returns what the caller may do as a member of the
role: [EffectivePermissions] for their credential, and for a token only
what the role it was minted under held as well.
*/
func PrincipalPermissions(catalog *auth.Catalog, role enums.Role, p *auth.Principal) []string {
	held := EffectivePermissions(catalog, role, p.Granted)
	if p.MintedAsRole == "" {
		return held
	}

	ceiling := auth.RolePermissions(p.MintedAsRole)

	return slices.DeleteFunc(held, func(permission string) bool { return !slices.Contains(ceiling, permission) })
}

/*
EffectiveAbilities returns the ability groups in which the permissions
hold at least one permission, in catalog order.
*/
func EffectiveAbilities(catalog *auth.Catalog, permissions []string) []string {
	return slices.DeleteFunc(auth.AbilityGroups(), func(group string) bool {
		return !slices.ContainsFunc(permissions, func(permission string) bool {
			held, _ := catalog.Group(permission)

			return held == group
		})
	})
}
