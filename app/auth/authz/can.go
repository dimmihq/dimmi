/*
|--------------------------------------------------------------------------
| Authorization
|--------------------------------------------------------------------------
|
| A caller's power in a workspace is computed on every request, never
| stored: the permissions of their role, narrowed to what their custom
| role lists when they hold one, and for a token only those whose
| ability group the token holds and that the role it was minted under
| held. Demoting a member or editing their custom role therefore shrinks
| every token they minted at once, a token can never do more than its
| member, and a promotion never widens a token minted before it.
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
alwaysHeld is the permission every member keeps, whatever their custom
role says: the app cannot even list the people it works with without it.
*/
const alwaysHeld = "members:read"

/*
HeldPermissions returns what a member of the role holds before any
credential narrows it, sorted. Custom is what the member's custom role
lists, nil when they hold none; it only ever narrows the role, so a key
the role does not hold, such as one left over from an older catalog, is
ignored.
*/
func HeldPermissions(role enums.Role, custom []string) []string {
	held := auth.RolePermissions(role)
	if custom == nil {
		return held
	}

	return slices.DeleteFunc(held, func(permission string) bool {
		return permission != alwaysHeld && !slices.Contains(custom, permission)
	})
}

/*
EffectivePermissions returns what a member of the role, narrowed to the
custom set when there is one, may do through a credential with the
abilities, sorted. Sessions and the user's own apps carry every ability,
so they get the member's permissions as they are.
*/
func EffectivePermissions(catalog *auth.Catalog, role enums.Role, custom, abilities []string) []string {
	groups := auth.ExpandAbilities(abilities)

	return slices.DeleteFunc(HeldPermissions(role, custom), func(permission string) bool {
		group, ok := catalog.Group(permission)

		return !ok || !slices.Contains(groups, group)
	})
}

/*
PrincipalPermissions returns what the caller may do as a member of the
role, narrowed to the custom set when there is one: [EffectivePermissions]
for their credential, and for a token only what the role it was minted
under held as well.
*/
func PrincipalPermissions(catalog *auth.Catalog, role enums.Role, custom []string, p *auth.Principal) []string {
	held := EffectivePermissions(catalog, role, custom, p.Granted)
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
