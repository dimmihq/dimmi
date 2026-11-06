/*
|--------------------------------------------------------------------------
| Send Password Reset Link
|--------------------------------------------------------------------------
|
| Someone who forgot their password asks for a link to choose a new
| one. The answer is the same whether or not the email has an account,
| so the form cannot be used to find out who signs in here. A user has
| one reset at a time: asking again replaces it, under a new id, so only
| the newest link works. Accounts without a password get a link too,
| which is how they add one.
|
| Asking too often is dropped silently rather than refused, since a
| refusal would tell an account exists.
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
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
resetTTL is how long a reset link works.
*/
const resetTTL = time.Hour

/*
SendPasswordResetLinkInput is the email typed and the client asking.
*/
type SendPasswordResetLinkInput struct {
	Email  string
	Client appauth.Client
}

/*
SendPasswordResetLink mails a live account a link to choose a new
password.

Rules:
  - Unknown emails get the same answer and no mail; an address blocked
    for guessing passwords across accounts is refused.
  - One request a minute and five an hour per email, twenty an hour per
    address; more are dropped silently.
  - A new request replaces the user's earlier reset and its id.
  - The link works for 60 minutes.

Errors: [exceptions.ErrTooManyAttempts], [exceptions.ErrAuthBusy], or a
wrapped error.
*/
type SendPasswordResetLink struct {
	DB      *database.DB
	Clock   clock.Clock
	Mail    *mail.Manager
	Limiter *ratelimit.Limiter
	Lockout *appauth.Lockout
	Links   appauth.Links
}

/*
Handle sends the link when the email has an account.
*/
func (a *SendPasswordResetLink) Handle(ctx context.Context, in SendPasswordResetLinkInput) error {
	email := mail.CanonicalAddress(in.Email)

	if err := a.Lockout.CheckAddress(ctx, in.Client.IP); err != nil {
		return err
	}

	allowed, err := appauth.Within(ctx, a.Limiter,
		appauth.Rate{Key: "reset:ip:" + in.Client.IP, Limit: 20, Window: time.Hour},
		appauth.Rate{Key: "reset:minute:" + email, Limit: 1, Window: time.Minute},
		appauth.Rate{Key: "reset:hour:" + email, Limit: 5, Window: time.Hour},
	)
	if err != nil || !allowed {
		return err
	}

	user, err := models.New(a.DB).FindUserByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find user to reset: %w", err)
	}

	return a.send(ctx, user, in.Client)
}

/*
send replaces the user's reset and queues the mail, in one transaction.
*/
func (a *SendPasswordResetLink) send(ctx context.Context, user models.User, client appauth.Client) error {
	now := a.Clock.Now()
	plain, digest := appauth.NewSecret()

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		reset, err := models.New(tx).ReplacePasswordResetToken(ctx, models.ReplacePasswordResetTokenParams{
			ID: ids.NewV7At(now), UserID: user.ID, TokenHash: digest, RequestedIp: appauth.Address(client.IP),
			ExpiresAt: now.Add(resetTTL), Now: now,
		})
		if err != nil {
			return fmt.Errorf("store password reset: %w", err)
		}

		return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.ResetPasswordMail{
			Name: user.Name, URL: a.Links.ResetPassword(reset.ID, plain),
		})
	})
}
