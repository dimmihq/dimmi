/*
|--------------------------------------------------------------------------
| Complete Primary Auth
|--------------------------------------------------------------------------
|
| Every way of proving who you are on its own, a password, a mailed link
| or code, a reset, ends here, so the rule about second factors lives in
| one place: an account protected by one is never signed in on the first
| proof alone, whichever proof it was.
|
*/

package auth

import (
	"context"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CompletePrimaryAuthInput is the user who just proved who they are, the
method they proved it with, and the client. Reauth marks a fresh
sign-in on an existing session.
*/
type CompletePrimaryAuthInput struct {
	User   models.User
	Method string
	Client appauth.Client
	Reauth bool
}

/*
CompletePrimaryAuth signs in a user who passed a primary method.

Rules:
  - A user with a second factor is not signed in until it can be asked
    for.
  - Otherwise a session starts with the method as its only one.

Errors: [exceptions.ErrTwoFactorUnavailable], or those of
[StartSession] and [LoadMe].
*/
type CompletePrimaryAuth struct {
	StartSession *StartSession
	LoadMe       *LoadMe
}

/*
Handle starts the session and describes the caller.
*/
func (a *CompletePrimaryAuth) Handle(ctx context.Context, in CompletePrimaryAuthInput) (SignedIn, error) {
	if in.User.TwoFactorEnabled() {
		return SignedIn{}, exceptions.ErrTwoFactorUnavailable
	}

	started, err := a.StartSession.Handle(ctx, StartSessionInput{
		User: in.User, Methods: []string{in.Method}, Client: in.Client, Reauth: in.Reauth,
	})
	if err != nil {
		return SignedIn{}, err
	}

	me, err := a.LoadMe.Handle(ctx, LoadMeInput{Principal: started.Principal})
	if err != nil {
		return SignedIn{}, err
	}

	return SignedIn{Me: me, DeviceToken: started.DeviceToken}, nil
}
