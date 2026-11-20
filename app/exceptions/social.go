/*
|--------------------------------------------------------------------------
| Messenger And Instagram Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a workspace connects a Facebook Page or an
| Instagram account. Each says what to do: sign in at Meta again, pick
| an account Meta shared, or wait for Meta.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Messenger and Instagram failures.

ErrSocialSignupUnavailable is a platform this install has no Meta app
or login for. ErrSocialSignupExpired is a sign-in at Meta that ran too
long or was already used. ErrSocialAccountNotFound is a Page or
Instagram account the sign-in did not share. ErrSocialUnavailable is
Meta not answering.
*/
var (
	ErrSocialSignupUnavailable = fault.New(fault.Invalid, "social_signup_unavailable",
		"This server has no Meta app set up for this channel.")
	ErrSocialSignupExpired = fault.New(fault.Invalid, "social_signup_expired",
		"The sign-in at Meta expired. Connect again.").OnField("code")
	ErrSocialAccountNotFound = fault.New(fault.Invalid, "social_account_not_found",
		"Meta did not share this account. Connect again and select it.").OnField("account_id")
	ErrSocialUnavailable = fault.New(fault.FailedDependency, "social_unavailable",
		"Meta could not be reached. Try again shortly.")
)
