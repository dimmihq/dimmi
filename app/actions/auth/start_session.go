/*
|--------------------------------------------------------------------------
| Start Session
|--------------------------------------------------------------------------
|
| Every successful sign-in, whatever the method, ends here. The browser
| gets a brand new session, so any session id planted in it before is
| worthless, and the session it came with is deleted. The session
| records how the user got in and from which device, which later checks
| read and which can never be recovered afterwards.
|
| A browser without a device cookie gets one, valid for a year, so it is
| recognized at the next sign-in: a known device keeps getting in while
| strangers are locked out of the account.
|
*/

package auth

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
errNoSession means the request carries no session to sign in, which is
a wiring mistake: sign-in routes always run the session middleware.
*/
var errNoSession = errors.New("start session: no session in context")

/*
StartSessionInput is who signed in, with which methods in order, and
from which client. Reauth marks a fresh sign-in on an existing session.
*/
type StartSessionInput struct {
	User    models.User
	Methods []string
	Client  appauth.Client
	Reauth  bool
}

/*
StartedSession is the caller as the new session, and the device token
to hand the browser when it had none.
*/
type StartedSession struct {
	Principal   *appauth.Principal
	DeviceToken string
}

/*
StartSession signs the user in on the request's session.

Rules:
  - The session gets a new id and token; the one the browser sent is
    deleted when the response goes out.
  - The full sign-in and the last confirmation are both now; the
    confirmation method is the last method passed.
  - A sign-in that cannot be logged is not started: the session is
    dropped again before the response goes out.

Errors: a wrapped database error from writing the security log.
*/
type StartSession struct {
	Devices     *appauth.Devices
	SecurityLog *appauth.SecurityLog
}

/*
Handle starts the session and logs the sign-in.
*/
func (a *StartSession) Handle(ctx context.Context, in StartSessionInput) (StartedSession, error) {
	s, ok := session.From(ctx)
	if !ok {
		return StartedSession{}, errNoSession
	}

	client, fresh := in.Client, ""
	if client.DeviceToken == "" {
		fresh = appauth.NewDeviceToken()
		client.DeviceToken = fresh
	}

	s.Login(in.User.ID, in.Methods...)
	s.SetDevice(client.DeviceID(), a.Devices.Label(client.UserAgent))
	signIn := appauth.SignInOf(s)

	err := a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventLoginSucceeded, Client: client,
		Metadata: map[string]any{
			"auth_methods": in.Methods, "session_id": models.PublicID(models.PrefixSession, s.ID()),
			"device_id": signIn.DeviceID, "reauth": in.Reauth,
		},
	})
	if err != nil {
		s.Destroy()

		return StartedSession{}, err
	}

	return StartedSession{
		Principal: &appauth.Principal{
			User: in.User, Kind: appauth.KindSession, CredentialID: s.ID(), Granted: []string{appauth.AllAbilities},
			SignIn: signIn, AuthenticatedAt: s.AuthenticatedAt(), Confirmed: s.ConfirmedAt(),
		},
		DeviceToken: fresh,
	}, nil
}
