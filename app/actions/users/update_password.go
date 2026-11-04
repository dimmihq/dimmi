/*
|--------------------------------------------------------------------------
| Update Password
|--------------------------------------------------------------------------
|
| Changing a password from inside the account. Someone with a password
| proves they know it; someone who signs in by link or provider has none
| to prove, so they confirm who they are first. Either way everything
| else signed in ends, while the browser or app asking stays signed in,
| its session moved to a new id. Access tokens keep working, because
| knowing the old password shows this is not a takeover, and the answer
| says how many still do. A reset, which may follow a takeover, ends them.
|
*/

package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdatePasswordInput is the caller, their current password when they
have one, the new one, and the client.
*/
type UpdatePasswordInput struct {
	Principal       *appauth.Principal
	CurrentPassword string
	Password        string
	Client          appauth.Client
}

/*
UpdatePassword sets the caller's own password.

Rules:
  - The login policy must allow a password at all.
  - With a password: the current one must be right, and a wrong one counts
    toward the lockout. Without one: a recent confirmation is needed.
  - A new password that is the account's email or name is refused.
  - Other sessions and first-party app grants end; the caller's own stays.

Events: [appevents.SessionRevoked] for each session that ended.

Errors: [exceptions.ErrCurrentPasswordRequired],
[exceptions.ErrWrongCurrentPassword], [exceptions.ErrPasswordTooCommon],
[exceptions.ErrSSORequired], [exceptions.ErrAuthBusy],
[exceptions.ErrTooManyAttempts], [fauth.ErrConfirmationRequired], or a
wrapped error.
*/
type UpdatePassword struct {
	DB            *database.DB
	Clock         clock.Clock
	Events        *events.Dispatcher
	Hash          *hashing.Manager
	Mail          *mail.Manager
	Links         appauth.Links
	LoginPolicy   appauth.LoginPolicyResolver
	Lockout       *appauth.Lockout
	Sessions      *session.Manager
	AppGrants     appauth.AppGrants
	SecurityLog   *appauth.SecurityLog
	ConfirmWindow time.Duration
}

/*
Handle sets the password and returns how many access tokens still work.
*/
func (a *UpdatePassword) Handle(ctx context.Context, in UpdatePasswordInput) (int64, error) {
	user := in.Principal.User

	err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, user.Email, appauth.MethodPassword)
	if err != nil {
		return 0, err
	}

	if err := a.prove(ctx, in); err != nil {
		return 0, err
	}

	if strings.EqualFold(in.Password, user.Email) || strings.EqualFold(in.Password, user.Name) {
		return 0, exceptions.ErrPasswordTooCommon
	}

	hash, err := a.Hash.Make(ctx, in.Password)
	if errors.Is(err, hashing.ErrBusy) {
		return 0, exceptions.ErrAuthBusy
	}

	if err != nil {
		return 0, fmt.Errorf("hash new password: %w", err)
	}

	var remaining int64

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		remaining, err = a.store(ctx, tx, in, hash)

		return err
	})
	if err != nil {
		return 0, err
	}

	if s, ok := session.From(ctx); ok && in.Principal.Kind == appauth.KindSession {
		s.Regenerate()
	}

	return remaining, nil
}

/*
prove checks the caller may set a new password: by the current one when
they have one, or else by a recent confirmation.
*/
func (a *UpdatePassword) prove(ctx context.Context, in UpdatePasswordInput) error {
	user := in.Principal.User
	if !user.HasPassword() {
		if !fauth.Confirmed(in.Principal, a.Clock.Now(), a.ConfirmWindow) {
			return fauth.ErrConfirmationRequired
		}

		return nil
	}

	if in.CurrentPassword == "" {
		return exceptions.ErrCurrentPasswordRequired
	}

	attempt := appauth.Attempt{IP: in.Client.IP, Email: user.Email, KnownDevice: true}
	if err := a.Lockout.Check(ctx, attempt); err != nil {
		return err
	}

	ok, err := a.Hash.Check(ctx, in.CurrentPassword, *user.PasswordHash)
	if errors.Is(err, hashing.ErrBusy) {
		return exceptions.ErrAuthBusy
	}

	if err != nil && !errors.Is(err, hashing.ErrInvalidHash) {
		return fmt.Errorf("check current password: %w", err)
	}

	if ok {
		return nil
	}

	if _, err := a.Lockout.Failed(ctx, attempt); err != nil {
		return err
	}

	return exceptions.ErrWrongCurrentPassword
}

/*
store writes the password, ends everything else signed in, logs it and
tells the person, returning how many access tokens still work.
*/
func (a *UpdatePassword) store(
	ctx context.Context, tx *database.Tx, in UpdatePasswordInput, hash string,
) (int64, error) {
	user, now := in.Principal.User, a.Clock.Now()
	q := models.New(tx)

	err := q.UpdateUserPassword(ctx, models.UpdateUserPasswordParams{ID: user.ID, PasswordHash: &hash, Now: now})
	if err != nil {
		return 0, fmt.Errorf("store new password: %w", err)
	}

	ended, err := a.endOthers(ctx, tx, in.Principal)
	if err != nil {
		return 0, err
	}

	remaining, err := q.CountUserLiveTokens(ctx, models.CountUserLiveTokensParams{UserID: user.ID, Now: now})
	if err != nil {
		return 0, fmt.Errorf("count tokens left: %w", err)
	}

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: appauth.EventPasswordChanged, Client: in.Client,
		Metadata: map[string]any{"sessions_ended": ended, "tokens_remaining": remaining},
	})
	if err != nil {
		return 0, err
	}

	return remaining, appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.PasswordChangedMail{
		Name: user.Name, Reason: appmail.PasswordWasChanged, SecurityURL: a.Links.Security(),
	})
}

/*
endOthers ends every session and first-party app grant but the caller's
own, announcing each session, and returns how many sessions ended.
*/
func (a *UpdatePassword) endOthers(ctx context.Context, tx *database.Tx, p *appauth.Principal) (int64, error) {
	keepSession, keepGrant := ids.Nil(), ids.Nil()

	switch p.Kind {
	case appauth.KindSession:
		keepSession = p.CredentialID
	case appauth.KindApp:
		keepGrant = p.CredentialID
	}

	live, err := a.Sessions.Sessions(ctx, p.ID())
	if err != nil {
		return 0, fmt.Errorf("list sessions to end: %w", err)
	}

	ended, err := a.Sessions.RevokeOthers(ctx, p.ID(), keepSession)
	if err != nil {
		return 0, fmt.Errorf("end other sessions: %w", err)
	}

	err = a.AppGrants.RevokeUserAppGrants(ctx, p.ID(), appauth.RevokedForPasswordChanged, keepGrant)
	if err != nil {
		return 0, fmt.Errorf("end other app grants: %w", err)
	}

	for _, s := range live {
		if s.ID == keepSession {
			continue
		}

		if err := a.Events.Dispatch(ctx, tx, appevents.SessionRevoked{UserID: p.ID(), SessionID: s.ID}); err != nil {
			return 0, err
		}
	}

	return ended, nil
}
