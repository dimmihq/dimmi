/*
|--------------------------------------------------------------------------
| Reset Password
|--------------------------------------------------------------------------
|
| Opening a reset link and choosing a new password. Every reset that
| cannot be used fails the same way and takes the same time: an unknown
| id, an id that is not a reset's at all, a wrong token, an expired
| link. A wrong token is counted, and the fifth burns the reset.
|
| A reset is a fresh start for the account: the new password is set,
| the inbox counts as proven, every browser session, app grant and access
| token ends, including the one that asked, and the user is told by mail.
| A reset may follow a compromise, so nothing a thief minted may outlive
| it. Only then is the user signed in, still through their second factor
| if they have one: a reset proves the inbox, never the second factor.
|
| An address nobody proved before is different: the reset is the owner's
| first proof, so the account is claimed from whoever registered it,
| second factors included, before the new password is set.
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
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ResetPasswordInput is the reset's public id and token from the link,
the new password, and the client.
*/
type ResetPasswordInput struct {
	ID       string
	Token    string
	Password string
	Client   appauth.Client
}

/*
ResetPassword sets a new password through a reset link and signs the
user in.

Rules:
  - Ten tries a minute per address.
  - Unknown, malformed, wrong or expired: invalid, with the same answer
    and the same work; five wrong tokens delete the reset.
  - The login policy is asked once the link proved the inbox.
  - An account never proven is claimed by [ClaimAccount] first, unless
    this browser is signed in to it.
  - The password is set, the email marked proven, the reset deleted,
    and every session, first-party app grant and access token of the
    user ended, in one transaction.
  - The user then signs in like after any primary method.

Events: [appevents.UserSignedOutEverywhere], and those of
[ClaimAccount].

Errors: [exceptions.ErrInvalidResetToken], [exceptions.ErrSSORequired],
[exceptions.ErrAuthBusy], [exceptions.ErrTooManyAttempts], those of
[CompletePrimaryAuth], or a wrapped error.
*/
type ResetPassword struct {
	DB                  *database.DB
	Clock               clock.Clock
	Events              *events.Dispatcher
	Hash                *hashing.Manager
	Mail                *mail.Manager
	Limiter             *ratelimit.Limiter
	IDs                 *ids.Registry
	Links               appauth.Links
	LoginPolicy         appauth.LoginPolicyResolver
	Sessions            *session.Manager
	AppGrants           appauth.AppGrants
	SecurityLog         *appauth.SecurityLog
	ClaimAccount        *ClaimAccount
	CompletePrimaryAuth *CompletePrimaryAuth
}

/*
Handle resets the password and signs the user in.
*/
func (a *ResetPassword) Handle(ctx context.Context, in ResetPasswordInput) (SignedIn, error) {
	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "reset:redeem:" + in.Client.IP, Limit: 10, Window: time.Minute,
	})
	if err != nil {
		return SignedIn{}, err
	}

	reset, user, err := a.find(ctx, in)
	if err != nil {
		return SignedIn{}, err
	}

	if err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, user.Email, appauth.MethodPasswordReset); err != nil {
		return SignedIn{}, err
	}

	hash, err := a.Hash.Make(ctx, in.Password)
	if errors.Is(err, hashing.ErrBusy) {
		return SignedIn{}, exceptions.ErrAuthBusy
	}

	if err != nil {
		return SignedIn{}, fmt.Errorf("hash new password: %w", err)
	}

	user.PasswordHash = &hash

	if err := a.reset(ctx, reset, user, in.Client); err != nil {
		return SignedIn{}, err
	}

	user, err = models.New(a.DB).FindUser(ctx, user.ID)
	if err != nil {
		return SignedIn{}, fmt.Errorf("reload reset user: %w", err)
	}

	return a.CompletePrimaryAuth.Handle(ctx, CompletePrimaryAuthInput{
		User: user, Method: appauth.MethodPasswordReset, Client: in.Client,
	})
}

/*
find returns the reset and its account when the link is good.

SECURITY: An unknown id is compared against a digest of nothing, so it
takes the same work as a wrong token for a real reset.
*/
func (a *ResetPassword) find(ctx context.Context, in ResetPasswordInput) (
	models.PasswordResetToken, models.User, error,
) {
	id, err := a.IDs.Decode(string(models.PrefixPasswordReset), in.ID)
	if err != nil {
		id = ids.Nil()
	}

	reset, err := models.New(a.DB).FindPasswordResetToken(ctx, id)
	known := err == nil

	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return models.PasswordResetToken{}, models.User{}, fmt.Errorf("find password reset: %w", err)
	}

	proves := hashing.Equal(appauth.Digest(in.Token), reset.TokenHash)
	if !known {
		return models.PasswordResetToken{}, models.User{}, exceptions.ErrInvalidResetToken
	}

	if !proves {
		return models.PasswordResetToken{}, models.User{}, a.miss(ctx, reset)
	}

	if !reset.ExpiresAt.After(a.Clock.Now()) {
		return models.PasswordResetToken{}, models.User{}, exceptions.ErrInvalidResetToken
	}

	user, err := models.New(a.DB).FindUser(ctx, reset.UserID)
	if err != nil {
		return models.PasswordResetToken{}, models.User{}, fmt.Errorf("find user to reset: %w", err)
	}

	if user.IsDeleted() {
		return models.PasswordResetToken{}, models.User{}, exceptions.ErrInvalidResetToken
	}

	return reset, user, nil
}

/*
miss counts a wrong token against the reset, deleting it at the fifth,
and returns the invalid answer.
*/
func (a *ResetPassword) miss(ctx context.Context, reset models.PasswordResetToken) error {
	attempts, err := models.New(a.DB).CountPasswordResetAttempt(ctx, reset.ID)
	if err != nil {
		return fmt.Errorf("count wrong reset token: %w", err)
	}

	if attempts < appauth.MaxSecretAttempts {
		return exceptions.ErrInvalidResetToken
	}

	if _, err := models.New(a.DB).DeletePasswordResetToken(ctx, reset.ID); err != nil {
		return fmt.Errorf("burn password reset: %w", err)
	}

	return exceptions.ErrInvalidResetToken
}

/*
reset spends the reset and gives the account, carrying its new hash,
its fresh start, in one transaction.
*/
func (a *ResetPassword) reset(
	ctx context.Context, reset models.PasswordResetToken, user models.User, client appauth.Client,
) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		spent, err := q.DeletePasswordResetToken(ctx, reset.ID)
		if err != nil {
			return fmt.Errorf("spend password reset: %w", err)
		}

		if spent == 0 {
			return exceptions.ErrInvalidResetToken
		}

		if !user.IsVerified() && !signedInAs(ctx, user) {
			if err := a.ClaimAccount.Wipe(ctx, tx, user, client); err != nil {
				return err
			}
		}

		if err := a.store(ctx, q, user); err != nil {
			return err
		}

		if err := a.endEverySession(ctx, tx, user, client); err != nil {
			return err
		}

		return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.PasswordChangedMail{
			Name: user.Name, Reason: appmail.PasswordWasReset, SecurityURL: a.Links.Security(),
		})
	})
}

/*
store sets the user's new password hash and marks the email proven.
*/
func (a *ResetPassword) store(ctx context.Context, q *models.Queries, user models.User) error {
	now := a.Clock.Now()

	err := q.UpdateUserPassword(ctx, models.UpdateUserPasswordParams{
		ID: user.ID, PasswordHash: user.PasswordHash, Now: now,
	})
	if err != nil {
		return fmt.Errorf("store new password: %w", err)
	}

	if err := q.MarkUserEmailVerified(ctx, models.MarkUserEmailVerifiedParams{ID: user.ID, Now: now}); err != nil {
		return fmt.Errorf("mark email verified by reset: %w", err)
	}

	return nil
}

/*
endEverySession ends every browser session, first-party app grant and
access token of the user, logs the reset with how many ended, and
announces it.
*/
func (a *ResetPassword) endEverySession(
	ctx context.Context, tx *database.Tx, user models.User, client appauth.Client,
) error {
	ended, err := a.Sessions.RevokeOthers(ctx, user.ID, ids.Nil())
	if err != nil {
		return fmt.Errorf("end sessions after reset: %w", err)
	}

	err = a.AppGrants.RevokeUserAppGrants(ctx, user.ID, appauth.RevokedForPasswordReset, ids.Nil())
	if err != nil {
		return fmt.Errorf("end app grants after reset: %w", err)
	}

	tokens, err := models.New(tx).RevokeUserTokens(ctx, models.RevokeUserTokensParams{
		UserID: user.ID, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("revoke tokens after reset: %w", err)
	}

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: appauth.EventPasswordReset, Client: client,
		Metadata: map[string]any{"sessions_ended": ended, "tokens_revoked": tokens},
	})
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, tx, appevents.UserSignedOutEverywhere{UserID: user.ID})
}
