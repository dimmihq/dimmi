/*
|--------------------------------------------------------------------------
| Signup Mode
|--------------------------------------------------------------------------
|
| Whether strangers may create accounts. An invitation for a person's
| own address always lets them in, whatever the mode.
|
*/

package enums

/*
SignupMode says who may create an account.
*/
type SignupMode string

/*
The signup modes.
*/
const (
	SignupModeOpen       SignupMode = "open"
	SignupModeInviteOnly SignupMode = "invite_only"
)

/*
Valid reports whether the mode is one of the modes.
*/
func (m SignupMode) Valid() bool {
	switch m {
	case SignupModeOpen, SignupModeInviteOnly:
		return true
	}

	return false
}
