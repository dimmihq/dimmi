/*
|--------------------------------------------------------------------------
| Abilities
|--------------------------------------------------------------------------
|
| Abilities narrow what a credential may do, never widen it. A route lists
| the abilities a token needs to call it, and a token issued with fewer
| is refused. A browser session holds every ability, so only tokens
| issued with a subset are ever stopped here; what a user may do at all
| is decided by the gate from their permissions.
|
*/

package auth

import (
	"context"
	"slices"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/routing"
)

/*
AllAbilities is the ability that stands for every ability.
*/
const AllAbilities = "*"

/*
ErrMissingAbility is the fault for a token that lacks an ability the
route requires. The response names the ability in required_ability.
*/
var ErrMissingAbility = fault.New(fault.Forbidden, "missing_ability",
	"The access token does not have an ability this route requires.")

/*
HasAbility reports whether the principal's credential carries the
ability, directly or through AllAbilities.
*/
func HasAbility(p Principal, ability string) bool {
	abilities := p.Abilities()

	return slices.Contains(abilities, AllAbilities) || slices.Contains(abilities, ability)
}

/*
checkAbilities returns ErrMissingAbility, naming the first missing one,
when the principal lacks an ability the matched route requires.
*/
func checkAbilities(ctx context.Context, p Principal) error {
	route, ok := routing.CurrentRoute(ctx)
	if !ok {
		return nil
	}

	for _, ability := range route.Abilities() {
		if !HasAbility(p, ability) {
			return ErrMissingAbility.With("required_ability", ability)
		}
	}

	return nil
}
