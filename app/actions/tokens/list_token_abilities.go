/*
|--------------------------------------------------------------------------
| List Token Abilities
|--------------------------------------------------------------------------
|
| Before minting a token a member picks its abilities. Every ability
| group is listed with the permissions it carries, and marked allowed
| only when the member's role holds at least one of them, so a client
| can grey out what minting would refuse anyway.
|
*/

package tokens

import (
	"slices"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
TokenAbility is one ability group: its name, the permissions it
carries, and whether the caller may give it to a token.
*/
type TokenAbility struct {
	Ability     string
	Permissions []string
	Allowed     bool
}

/*
ListTokenAbilities lists the ability groups for the caller's role.
*/
type ListTokenAbilities struct {
	Permissions *appauth.Catalog
}

/*
Handle returns every ability group in catalog order.
*/
func (a *ListTokenAbilities) Handle(m authz.Membership) []TokenAbility {
	allowed := authz.EffectiveAbilities(a.Permissions, m.Principal.Permissions)
	all := a.Permissions.Permissions()
	groups := appauth.AbilityGroups()
	out := make([]TokenAbility, 0, len(groups))

	for _, group := range groups {
		permissions := slices.DeleteFunc(slices.Clone(all), func(permission string) bool {
			held, _ := a.Permissions.Group(permission)

			return held != group
		})

		out = append(out, TokenAbility{
			Ability: group, Permissions: permissions, Allowed: slices.Contains(allowed, group),
		})
	}

	return out
}
