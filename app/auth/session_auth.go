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
	"time"

	"github.com/dimmihq/dimmi/framework/session"
)

/*
workedAtKey is the session value holding when its user last worked in a
workspace.
*/
const workedAtKey = "worked_at"

/*
workedAtInterval is how often a session writes that it is still at work.
A workspace's idle limit can run this much over; the settings page says
so.
*/
const workedAtInterval = 5 * time.Minute

/*
WorkedAt returns the last time the session's user worked in a workspace,
or the last full sign-in when they have not yet. The session's own last
activity cannot answer this: every request moves it, the ones that ask
who is signed in or that a workspace refuses among them, so an idle
session would look busy again the moment the page came back.
*/
func WorkedAt(r session.Record) time.Time {
	at, err := time.Parse(time.RFC3339, r.Data[workedAtKey])
	if err != nil {
		return r.AuthenticatedAt
	}

	return at
}

/*
MarkWorked records that the session's user just worked in a workspace,
at most once per interval.
*/
func MarkWorked(s *session.Session, now time.Time) {
	if value, ok := s.Get(workedAtKey); ok {
		if at, err := time.Parse(time.RFC3339, value); err == nil && now.Sub(at) < workedAtInterval {
			return
		}
	}

	s.Put(workedAtKey, now.UTC().Format(time.RFC3339))
}

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
