/*
|--------------------------------------------------------------------------
| Confirm User
|--------------------------------------------------------------------------
|
| Before a change that could lock the owner out, such as minting a token
| or ending every other session, the caller proves again who they are:
| a stolen session alone should not be enough. A user with a password
| types it; a user without one types a code mailed to them. A wrong
| answer counts toward the same lockout as a wrong sign-in, so the
| prompt cannot be used to guess a password.
|
| Confirming is not signing in: the session keeps when and how it
| signed in, and only records when and how the user last confirmed.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
ConfirmUserInput is the caller, the method they confirm with and the
password or code they typed, and the client.
*/
type ConfirmUserInput struct {
	Principal *appauth.Principal
	Method    string
	Password  string
	Code      string
	Client    appauth.Client
}

/*
Confirmation is how the caller last confirmed who they are and until
when that holds.
*/
type Confirmation struct {
	Method string
	Until  time.Time
}

/*
ConfirmUser records that the caller proved who they are again.

Rules:
  - A password confirms a user who has one; an email code a user who
    has none. Any other method is not available.
  - The lockout is checked first, and a wrong answer counts toward it.
  - The session's sign-in time stays as it was; the confirmation holds
    for TTL from the moment the session recorded it.

Errors: [exceptions.ErrMethodNotAvailable], [exceptions.ErrWrongPassword],
[exceptions.ErrInvalidCode], [exceptions.ErrTooManyAttempts],
[exceptions.ErrAuthBusy], or a wrapped error.
*/
type ConfirmUser struct {
	Hash    *hashing.Manager
	Lockout *appauth.Lockout
	Tokens  *appauth.OneTimeTokens
	TTL     time.Duration
}

/*
Handle checks the answer and records the confirmation on the session.
*/
func (a *ConfirmUser) Handle(ctx context.Context, in ConfirmUserInput) (Confirmation, error) {
	check, ok := a.checker(in)
	if !ok {
		return Confirmation{}, exceptions.ErrMethodNotAvailable
	}

	attempt := appauth.Attempt{IP: in.Client.IP, Email: in.Principal.User.Email, KnownDevice: true}
	if err := a.Lockout.Check(ctx, attempt); err != nil {
		return Confirmation{}, err
	}

	if err := check(ctx, in); err != nil {
		return Confirmation{}, a.fail(ctx, attempt, err)
	}

	s, _ := session.From(ctx)
	s.Confirm(in.Method)

	return Confirmation{Method: in.Method, Until: s.ConfirmedAt().Add(a.TTL)}, nil
}

/*
checker returns the check for the method, when the caller can confirm
with it: a password for a user who has one, a mailed code for a user
who has none.
*/
func (a *ConfirmUser) checker(in ConfirmUserInput) (func(context.Context, ConfirmUserInput) error, bool) {
	hasPassword := in.Principal.User.HasPassword()

	switch in.Method {
	case appauth.MethodPassword:
		return a.checkPassword, hasPassword
	case appauth.MethodEmailCode:
		return a.checkCode, !hasPassword
	default:
		return nil, false
	}
}

/*
checkPassword checks the caller's password.
*/
func (a *ConfirmUser) checkPassword(ctx context.Context, in ConfirmUserInput) error {
	ok, err := a.Hash.Check(ctx, in.Password, *in.Principal.User.PasswordHash)
	if errors.Is(err, hashing.ErrBusy) {
		return exceptions.ErrAuthBusy
	}

	if err != nil && !errors.Is(err, hashing.ErrInvalidHash) {
		return fmt.Errorf("check password to confirm: %w", err)
	}

	if !ok {
		return exceptions.ErrWrongPassword
	}

	return nil
}

/*
checkCode spends the confirmation code mailed to the caller.
*/
func (a *ConfirmUser) checkCode(ctx context.Context, in ConfirmUserInput) error {
	tok, ok, err := a.Tokens.MatchCode(ctx, enums.OneTimePurposeConfirmCode, in.Principal.User.Email, in.Code)
	if err != nil {
		return err
	}

	if !ok {
		return exceptions.ErrInvalidCode
	}

	spent, err := a.Tokens.Spend(ctx, tok)
	if err != nil {
		return err
	}

	if !spent {
		return exceptions.ErrInvalidCode
	}

	return nil
}

/*
fail counts a wrong answer toward the lockout and returns why it
failed. A busy server counts nothing.
*/
func (a *ConfirmUser) fail(ctx context.Context, attempt appauth.Attempt, reason error) error {
	if !errors.Is(reason, exceptions.ErrWrongPassword) && !errors.Is(reason, exceptions.ErrInvalidCode) {
		return reason
	}

	if _, err := a.Lockout.Failed(ctx, attempt); err != nil {
		return err
	}

	return reason
}
