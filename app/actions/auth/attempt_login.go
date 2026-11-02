/*
|--------------------------------------------------------------------------
| Attempt Login
|--------------------------------------------------------------------------
|
| Signing in with an email and a password. The answer must never tell an
| attacker whether an account exists: an unknown email, an account
| without a password and a wrong password all fail the same way, after
| the same expensive hash, so even the timing is alike.
|
| The order of checks is the point. The login policy is asked first, then
| the lockout, both before any hash, so a locked or refused attempt costs
| the server nothing and a correct password gets no further than a wrong
| one while locked. Only then is the password checked; a hash made with
| older settings is replaced on the spot.
|
| A sign-in on a session that already has a user is a re-authentication
| when asked for: it must prove the same user, and a different one fails
| like a wrong password.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AttemptLoginInput is what the person typed and where they typed it.
Reauth asks for a fresh sign-in of the session's own user.
*/
type AttemptLoginInput struct {
	Email    string
	Password string
	Reauth   bool
	Client   appauth.Client
}

/*
SignedIn is the caller after signing in, and the device token to hand
the browser when it had none.
*/
type SignedIn struct {
	Me          Me
	DeviceToken string
}

/*
AttemptLogin signs a user in with their password.

Rules:
  - The login policy and the lockout are checked before the password.
  - Unknown email, no password and wrong password fail alike, after a
    password hash, and count toward the lockout.
  - With Reauth on a signed-in session, the password must belong to the
    session's user.
  - A user with a second factor is not signed in until it can be asked
    for.

Errors: [exceptions.ErrInvalidCredentials],
[exceptions.ErrTooManyAttempts], [exceptions.ErrReauthUserMismatch],
[exceptions.ErrSSORequired], [exceptions.ErrAuthBusy],
[exceptions.ErrTwoFactorUnavailable], or a wrapped error.
*/
type AttemptLogin struct {
	DB           *database.DB
	Hash         *hashing.Manager
	Clock        clock.Clock
	LoginPolicy  appauth.LoginPolicyResolver
	Lockout      *appauth.Lockout
	SecurityLog  *appauth.SecurityLog
	StartSession *StartSession
	LoadMe       *LoadMe
	DummyHash    string
}

/*
signInTry is one sign-in under way: the lockout's view of it, the
account the email names, nil when none does, and the client.
*/
type signInTry struct {
	attempt appauth.Attempt
	account *models.User
	client  appauth.Client
}

/*
Handle checks the credentials and starts a session.
*/
func (a *AttemptLogin) Handle(ctx context.Context, in AttemptLoginInput) (SignedIn, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))

	if err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, email, appauth.MethodPassword); err != nil {
		return SignedIn{}, err
	}

	try, err := a.begin(ctx, email, in.Client)
	if err != nil {
		return SignedIn{}, err
	}

	if err := a.Lockout.Check(ctx, try.attempt); err != nil {
		return SignedIn{}, a.refuse(ctx, try, err)
	}

	if err := a.verify(ctx, try, in.Password); err != nil {
		return SignedIn{}, a.fail(ctx, try, err)
	}

	if in.Reauth && !isSessionUser(ctx, *try.account) {
		return SignedIn{}, a.fail(ctx, try, exceptions.ErrReauthUserMismatch)
	}

	return a.succeed(ctx, try, in)
}

/*
begin finds the account the email names and describes the try for the
lockout, telling whether this browser signed in as this account before.
*/
func (a *AttemptLogin) begin(ctx context.Context, email string, client appauth.Client) (signInTry, error) {
	try := signInTry{attempt: appauth.Attempt{IP: client.IP, Email: email}, client: client}

	user, err := models.New(a.DB).FindUserByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return try, nil
	}

	if err != nil {
		return try, fmt.Errorf("find user to sign in: %w", err)
	}

	try.account = &user

	if client.DeviceID() == "" {
		return try, nil
	}

	try.attempt.KnownDevice, err = models.New(a.DB).HasSignedInFromDevice(ctx, models.HasSignedInFromDeviceParams{
		UserID: user.ID, DeviceID: client.DeviceID(),
	})
	if err != nil {
		return try, fmt.Errorf("check known device: %w", err)
	}

	return try, nil
}

/*
verify checks the password, hashing against a dummy hash when there is
no password to check, so every failure takes as long as a real one.

A stored hash no hasher can read fails like a wrong password: the
account cannot be proven, and saying so would tell the hash is broken.

SECURITY: Never return before hashing; the time an answer takes would
tell which emails have accounts.
*/
func (a *AttemptLogin) verify(ctx context.Context, try signInTry, password string) error {
	hasPassword := try.account != nil && try.account.HasPassword()

	hash := a.DummyHash
	if hasPassword {
		hash = *try.account.PasswordHash
	}

	ok, err := a.Hash.Check(ctx, password, hash)
	if errors.Is(err, hashing.ErrBusy) {
		return exceptions.ErrAuthBusy
	}

	if errors.Is(err, hashing.ErrInvalidHash) {
		return exceptions.ErrInvalidCredentials
	}

	if err != nil {
		return fmt.Errorf("check password: %w", err)
	}

	if !ok || !hasPassword {
		return exceptions.ErrInvalidCredentials
	}

	return nil
}

/*
fail counts the failure toward the lockout and logs it for an existing
account, then returns the reason. A busy server counts nothing.
*/
func (a *AttemptLogin) fail(ctx context.Context, try signInTry, reason error) error {
	if errors.Is(reason, exceptions.ErrAuthBusy) {
		return reason
	}

	locked, err := a.Lockout.Failed(ctx, try.attempt)
	if err != nil {
		return err
	}

	event := appauth.EventLoginFailed
	if locked {
		event = appauth.EventLoginLocked
	}

	if err := a.log(ctx, try, event); err != nil {
		return err
	}

	return reason
}

/*
refuse logs a locked attempt on an existing account and returns the
refusal.
*/
func (a *AttemptLogin) refuse(ctx context.Context, try signInTry, refusal error) error {
	if !errors.Is(refusal, exceptions.ErrTooManyAttempts) {
		return refusal
	}

	if err := a.log(ctx, try, appauth.EventLoginLocked); err != nil {
		return err
	}

	return refusal
}

/*
log writes a failed or locked sign-in to the account's security log. A
try at an email without an account has no log to write to.
*/
func (a *AttemptLogin) log(ctx context.Context, try signInTry, event string) error {
	if try.account == nil {
		return nil
	}

	return a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: try.account.ID, Type: event, Client: try.client,
		Metadata: map[string]any{"method": appauth.MethodPassword},
	})
}

/*
succeed ends the run of failures, refreshes an outdated hash and starts
the session.
*/
func (a *AttemptLogin) succeed(ctx context.Context, try signInTry, in AttemptLoginInput) (SignedIn, error) {
	user := *try.account
	if user.TwoFactorEnabled() {
		return SignedIn{}, exceptions.ErrTwoFactorUnavailable
	}

	if err := a.Lockout.Succeeded(ctx, try.attempt); err != nil {
		return SignedIn{}, err
	}

	if err := a.rehash(ctx, user, in.Password); err != nil {
		return SignedIn{}, err
	}

	started, err := a.StartSession.Handle(ctx, StartSessionInput{
		User: user, Methods: []string{appauth.MethodPassword}, Client: in.Client, Reauth: in.Reauth,
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

/*
rehash replaces a hash made with older settings by one made with the
current ones, while the password is at hand.
*/
func (a *AttemptLogin) rehash(ctx context.Context, user models.User, password string) error {
	if !a.Hash.NeedsRehash(*user.PasswordHash) {
		return nil
	}

	hash, err := a.Hash.Make(ctx, password)
	if err != nil {
		return fmt.Errorf("rehash password: %w", err)
	}

	err = models.New(a.DB).UpdateUserPasswordHash(ctx, models.UpdateUserPasswordHashParams{
		ID: user.ID, PasswordHash: &hash, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("store rehashed password: %w", err)
	}

	return nil
}

/*
isSessionUser reports whether the request's session is signed in as the
user, or signed in as nobody, in which case a re-authentication is just
a sign-in.
*/
func isSessionUser(ctx context.Context, user models.User) bool {
	s, ok := session.From(ctx)
	if !ok {
		return true
	}

	current, ok := s.UserID()

	return !ok || current == user.ID
}
