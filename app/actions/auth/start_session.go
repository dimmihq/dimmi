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
| strangers are locked out of the account. A sign-in from a device the
| account has not seen for 90 days is mailed to its owner, except the
| very first one after signing up and a fresh sign-in on a session that
| was already theirs.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
deviceMemory is how long a device that signed in stays known: a sign-in
from a device not seen for longer is mailed as new.
*/
const deviceMemory = 90 * 24 * time.Hour

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
  - A sign-in from a new device is mailed to the user, unless it is
    their first sign-in or a fresh sign-in on their own session.

Errors: a wrapped database error from reading the sign-in history,
writing the security log or queueing the mail.
*/
type StartSession struct {
	DB          *database.DB
	Clock       clock.Clock
	Mail        *mail.Manager
	Links       appauth.Links
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

	fresh := ""
	if in.Client.DeviceToken == "" {
		fresh = appauth.NewDeviceToken()
		in.Client.DeviceToken = fresh
	}

	newDevice, err := a.isNewDevice(ctx, in)
	if err != nil {
		return StartedSession{}, err
	}

	s.Login(in.User.ID, in.Methods...)
	s.SetDevice(in.Client.DeviceID(), a.Devices.Label(in.Client.UserAgent))
	signIn := appauth.SignInOf(s)

	if err := a.record(ctx, s, in); err != nil {
		s.Destroy()

		return StartedSession{}, err
	}

	if newDevice {
		if err := a.notify(ctx, in, signIn); err != nil {
			s.Destroy()

			return StartedSession{}, err
		}
	}

	return StartedSession{
		Principal: &appauth.Principal{
			User: in.User, Kind: appauth.KindSession, CredentialID: s.ID(), Granted: []string{appauth.AllAbilities},
			SignIn: signIn, AuthenticatedAt: s.AuthenticatedAt(), Confirmed: s.ConfirmedAt(),
		},
		DeviceToken: fresh,
	}, nil
}

/*
isNewDevice reports whether the sign-in should be mailed as one from a
new device: the user signed in before, never from this device in the
last 90 days, and is not signing in afresh on their own session.
*/
func (a *StartSession) isNewDevice(ctx context.Context, in StartSessionInput) (bool, error) {
	if in.Reauth {
		return false, nil
	}

	history, err := models.New(a.DB).FindSignInHistory(ctx, models.FindSignInHistoryParams{
		UserID: in.User.ID, Since: a.Clock.Now().Add(-deviceMemory), DeviceID: in.Client.DeviceID(),
	})
	if err != nil {
		return false, fmt.Errorf("read sign-in history: %w", err)
	}

	return history.SignedInBefore && !history.KnownDevice, nil
}

/*
record logs the sign-in.
*/
func (a *StartSession) record(ctx context.Context, s *session.Session, in StartSessionInput) error {
	return a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventLoginSucceeded, Client: in.Client,
		Metadata: map[string]any{
			"auth_methods": in.Methods, "session_id": models.PublicID(models.PrefixSession, s.ID()),
			"device_id": in.Client.DeviceID(), "reauth": in.Reauth,
		},
	})
}

/*
notify mails the user a sign-in from a new device.
*/
func (a *StartSession) notify(ctx context.Context, in StartSessionInput, signIn appauth.SignIn) error {
	err := appmail.ToUser(a.Mail, in.User).Queue(ctx, appmail.NewSignInMail{
		Name: in.User.Name, Device: signIn.DeviceLabel, IP: in.Client.IP, SecurityURL: a.Links.Security(),
	})
	if err != nil {
		return fmt.Errorf("queue new sign-in mail: %w", err)
	}

	return nil
}
