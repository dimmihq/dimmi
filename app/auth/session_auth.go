/*
|--------------------------------------------------------------------------
| Session Sign-In Record
|--------------------------------------------------------------------------
|
| A session remembers how its user got in: the primary method, every
| method of the last full sign-in, the device, and how the user last
| confirmed who they are. The framework session keeps that record in
| columns of its own; this is the product's reading of it, which knows
| which methods count as a second factor.
|
*/

package auth

import (
	"github.com/dimmihq/dimmi/framework/session"
)

/*
SignIn is how a session's user signed in. Method is the primary method
and Methods every method passed, the primary one first, such as
password then totp. ConfirmedMethod is how they last proved who they
are, which a full sign-in sets to its last method.
*/
type SignIn struct {
	Method          string
	Methods         []string
	DeviceID        string
	DeviceLabel     string
	ConfirmedMethod string
}

/*
UsedSecondFactor reports whether the sign-in passed a second factor.
*/
func (s SignIn) UsedSecondFactor() bool {
	for _, method := range s.Methods {
		if SecondFactors().Has(method) {
			return true
		}
	}

	return false
}

/*
SignInOf returns the sign-in recorded in the session.
*/
func SignInOf(s *session.Session) SignIn {
	return SignInFrom(s.Record())
}

/*
SignInFrom reads the sign-in out of a stored session's record.
*/
func SignInFrom(r session.Record) SignIn {
	return SignIn{
		Method:          r.AuthMethod,
		Methods:         r.AuthMethods,
		DeviceID:        r.DeviceID,
		DeviceLabel:     r.DeviceLabel,
		ConfirmedMethod: r.ConfirmedMethod,
	}
}
