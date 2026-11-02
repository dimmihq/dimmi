/*
|--------------------------------------------------------------------------
| One-Time Secret Purpose
|--------------------------------------------------------------------------
|
| A one-time secret is good for exactly one purpose, so a link mailed to
| verify an address can never be replayed to sign in.
|
*/

package enums

/*
OneTimePurpose is what a one-time secret, such as a mailed link, may be
spent on.
*/
type OneTimePurpose string

/*
The one-time secret purposes.
*/
const (
	OneTimePurposeMagicLink          OneTimePurpose = "magic_link"
	OneTimePurposeTwoFactorChallenge OneTimePurpose = "two_factor_challenge"
	OneTimePurposeEmailVerification  OneTimePurpose = "email_verification"
	OneTimePurposeEmailChange        OneTimePurpose = "email_change"
	OneTimePurposeConfirmCode        OneTimePurpose = "confirm_code"
)

/*
OneTimePurposes returns every purpose, in the order of the database
check.
*/
func OneTimePurposes() []OneTimePurpose {
	return []OneTimePurpose{
		OneTimePurposeMagicLink, OneTimePurposeTwoFactorChallenge, OneTimePurposeEmailVerification,
		OneTimePurposeEmailChange, OneTimePurposeConfirmCode,
	}
}

/*
Valid reports whether the purpose is one of the purposes.
*/
func (p OneTimePurpose) Valid() bool {
	switch p {
	case OneTimePurposeMagicLink, OneTimePurposeTwoFactorChallenge, OneTimePurposeEmailVerification,
		OneTimePurposeEmailChange, OneTimePurposeConfirmCode:
		return true
	}

	return false
}
