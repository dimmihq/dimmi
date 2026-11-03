/*
|--------------------------------------------------------------------------
| Second Factor Records
|--------------------------------------------------------------------------
|
| What the security log records when a user's second factors change or
| are used, and why their app grants end when two-factor authentication
| is turned off. Entries about a factor name it by its public id and its
| type, never by anything secret.
|
*/

package auth

/*
The security events of second factors: the account's two-factor state,
and changes to and uses of single factors.
*/
const (
	EventTwoFactorEnabled               = "two_factor.enabled"
	EventTwoFactorDisabled              = "two_factor.disabled"
	EventAuthFactorAdded                = "auth_factor.added"
	EventAuthFactorRemoved              = "auth_factor.removed"
	EventAuthFactorRenamed              = "auth_factor.renamed"
	EventAuthFactorRecoveryCodeUsed     = "auth_factor.recovery_code_used"
	EventAuthFactorRecoveryCodesRenewed = "auth_factor.recovery_codes_regenerated"
)

/*
RevokedForTwoFactorDisabled is why first-party app grants end when the
user turns two-factor authentication off: they were granted under the
protection the user just removed.
*/
const RevokedForTwoFactorDisabled = "two_factor_disabled"
