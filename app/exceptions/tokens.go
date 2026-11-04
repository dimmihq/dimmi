/*
|--------------------------------------------------------------------------
| Personal Access Token Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a member mints and manages their tokens. A
| token never reaches further than its member's role, so asking for an
| ability the role holds nothing of is refused up front rather than
| minting a token that silently does less than its name promises.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Token failures.

ErrAbilityNotAllowed refuses an ability group in which the member's
role holds no permission; callers add the offending groups.
ErrTokenExpiryInvalid refuses an expiry that is not in the future or is
more than five years away. ErrTokenLimitReached refuses a token beyond
how many live tokens a member may hold.
*/
var (
	ErrAbilityNotAllowed = fault.New(fault.Invalid, "ability_not_allowed",
		"Your role cannot use some of these abilities.").OnField("abilities")
	ErrTokenExpiryInvalid = fault.New(fault.Invalid, "invalid_expiry",
		"Choose an expiry date in the future and at most five years away.").OnField("expires_at")
	ErrTokenLimitReached = fault.New(fault.Conflict, "token_limit_reached",
		"You hold as many tokens as you may. Revoke one you no longer use first.")
)
