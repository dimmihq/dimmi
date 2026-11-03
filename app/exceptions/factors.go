/*
|--------------------------------------------------------------------------
| Auth Factor Errors
|--------------------------------------------------------------------------
|
| What can go wrong while answering the second-factor challenge and while
| a user manages their factors. Codes never change once shipped: apps
| branch on them, such as sending the person back to sign in when the
| challenge ran out.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
ErrTwoFactorChallengeExpired means the browser holds no live challenge:
it never had one, it ran out, too many wrong codes burned it, or it was
already answered. The person signs in again.
*/
var ErrTwoFactorChallengeExpired = fault.New(fault.Unauthenticated, "two_factor_challenge_expired",
	"The sign-in took too long or had too many wrong codes. Sign in again.")

/*
Factor management failures.

ErrFactorTypeNotAvailable refuses a kind of factor the install does not
offer. ErrFactorLimitReached refuses a factor beyond what a user may hold: a
second of a kind held once, or more than the install allows.
ErrInvalidTransition refuses activating a factor that is already
active. ErrPreferredNotAllowed refuses making recovery codes the
preferred factor, since they only stand in for another.
ErrCodeProhibited refuses a code sent along with other changes, since
activating is a step of its own. ErrTwoFactorNotEnabled refuses new
recovery codes for an account without two-factor authentication.
ErrRecoveryCodesRequired refuses removing the recovery codes while
two-factor authentication stays on: they are the way back in when the
authenticator is lost.
*/
var (
	ErrFactorTypeNotAvailable = fault.New(fault.Invalid, "factor_type_not_available",
		"This kind of factor is not available.").OnField("type")
	ErrFactorLimitReached = fault.New(fault.Invalid, "factor_limit_reached",
		"You cannot add more factors of this kind.").OnField("type")
	ErrInvalidTransition = fault.New(fault.Invalid, "invalid_transition",
		"This factor is already active.").OnField("code").With("from", "active").With("to", "active")
	ErrPreferredNotAllowed = fault.New(fault.Invalid, "preferred_not_allowed",
		"Recovery codes cannot be your preferred factor.").OnField("preferred")
	ErrCodeProhibited = fault.New(fault.Invalid, "prohibited",
		"The code cannot be sent with other changes.").OnField("name")
	ErrTwoFactorNotEnabled = fault.New(fault.Conflict, "two_factor_not_enabled",
		"Turn on two-factor authentication first.")
	ErrRecoveryCodesRequired = fault.New(fault.Conflict, "recovery_codes_required",
		"Recovery codes stay while two-factor authentication is on. Regenerate them instead.")
)
