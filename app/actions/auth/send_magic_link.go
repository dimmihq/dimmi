/*
|--------------------------------------------------------------------------
| Send Magic Link
|--------------------------------------------------------------------------
|
| Signing in, or signing up, without a password: a link and a six-digit
| code are mailed to the address. The link suits the device that reads
| the mail, the code the device that asked. The answer is the same for
| every address, whether it has an account, may sign up, or neither, so
| the form says nothing about who signs in here; asking too often is
| dropped silently for the same reason.
|
| An address without an account gets a link only where anyone may sign
| up; following it creates the account. A few links may be live at once
| for one address, so several tabs keep working, and asking again ends
| the oldest.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
magicLinksLive is how many sign-in links may be live for one address.
*/
const magicLinksLive = 3

/*
SendMagicLinkInput is the email typed, the name to give a new account,
and the client asking.
*/
type SendMagicLinkInput struct {
	Email  string
	Name   string
	Client appauth.Client
}

/*
SendMagicLink mails a sign-in link and code.

Rules:
  - Turned off by configuration: not found.
  - The login policy for the address is asked first; an address blocked
    for guessing passwords across accounts is refused.
  - Three requests per ten minutes per email, twenty an hour per
    address; more are dropped silently.
  - An existing account gets a link; an unknown address gets one only
    when anyone may sign up or an invitation waits for it, and nothing
    otherwise.
  - At most three links work at once per address.

Errors: [exceptions.ErrNotFound], [exceptions.ErrSSORequired],
[exceptions.ErrTooManyAttempts], [exceptions.ErrAuthBusy], or a wrapped
error.
*/
type SendMagicLink struct {
	DB          *database.DB
	Clock       clock.Clock
	Mail        *mail.Manager
	Limiter     *ratelimit.Limiter
	Links       appauth.Links
	Devices     *appauth.Devices
	Tokens      *appauth.OneTimeTokens
	LoginPolicy appauth.LoginPolicyResolver
	Lockout     *appauth.Lockout
	MagicLink   config.MagicLinkConfig
	Signup      config.SignupConfig
}

/*
Handle sends the link when the address may sign in or sign up.
*/
func (a *SendMagicLink) Handle(ctx context.Context, in SendMagicLinkInput) error {
	if !a.MagicLink.Enabled {
		return exceptions.ErrNotFound
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))

	if err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, email, appauth.MethodMagicLink); err != nil {
		return err
	}

	if err := a.Lockout.CheckAddress(ctx, in.Client.IP); err != nil {
		return err
	}

	allowed, err := appauth.Within(ctx, a.Limiter,
		appauth.Rate{Key: "magic:ip:" + in.Client.IP, Limit: 20, Window: time.Hour},
		appauth.Rate{Key: "magic:email:" + email, Limit: magicLinksLive, Window: 10 * time.Minute},
	)
	if err != nil || !allowed {
		return err
	}

	user, err := models.New(a.DB).FindUserByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return a.sendToNewcomer(ctx, email, in)
	}

	if err != nil {
		return fmt.Errorf("find user for magic link: %w", err)
	}

	return a.send(ctx, appmail.ToUser(a.Mail, user), appauth.Issue{UserID: &user.ID, Email: user.Email}, in.Client)
}

/*
sendToNewcomer mails a sign-up link to an address without an account,
remembering the name to give it, only where anyone may sign up or a
pending invitation waits for the address.
*/
func (a *SendMagicLink) sendToNewcomer(ctx context.Context, email string, in SendMagicLinkInput) error {
	if a.Signup.Mode != config.SignupOpen {
		pending := models.CountPendingInvitationsForEmailParams{Email: email, Now: a.Clock.Now()}

		invited, err := models.New(a.DB).CountPendingInvitationsForEmail(ctx, pending)
		if err != nil {
			return fmt.Errorf("count pending invitations: %w", err)
		}

		if invited == 0 {
			return nil
		}
	}

	pending := a.Mail.To(mail.Address{Email: strings.TrimSpace(in.Email)})

	return a.send(ctx, pending, appauth.Issue{Email: email, Data: map[string]any{"name": in.Name}}, in.Client)
}

/*
send issues the link and code for the issue's address and queues the
mail to it, in one transaction.
*/
func (a *SendMagicLink) send(
	ctx context.Context, to *mail.PendingMail, issue appauth.Issue, client appauth.Client,
) error {
	issue.Purpose, issue.Client, issue.TTL = enums.OneTimePurposeMagicLink, client, a.MagicLink.TTL
	issue.WithCode, issue.Live = true, magicLinksLive

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		issued, err := a.Tokens.Issue(ctx, issue)
		if err != nil {
			return err
		}

		return to.QueueTx(ctx, tx, appmail.MagicLinkMail{
			URL: a.Links.MagicLink(issued.Token), Code: appauth.FormatCode(issued.Code), IP: client.IP,
			Device: a.Devices.Label(client.UserAgent), Minutes: int(a.MagicLink.TTL.Minutes()),
		})
	})
}
