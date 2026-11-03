/*
|--------------------------------------------------------------------------
| Send Email Verification
|--------------------------------------------------------------------------
|
| A new account proves its address by opening a link mailed to it. The
| link works from any device for a day; a person who asks again gets a
| fresh link while the last few keep working, so the one in an older
| tab does too. The link is bound to the address it was sent to, so it
| can never prove an address the account has since moved away from.
|
*/

package auth

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long a verification link works, and how many may work at once.
*/
const (
	verificationTTL  = 24 * time.Hour
	verificationLive = 3
)

/*
SendEmailVerification mails the user a link that proves their address.

Rules:
  - The link works for 24 hours, for the address the user has now.
  - At most three links work at once; the oldest stops first.
  - The mail is queued with the link, so a failed send never loses it.

Errors: a wrapped database error.
*/
type SendEmailVerification struct {
	DB     *database.DB
	Mail   *mail.Manager
	Links  appauth.Links
	Tokens *appauth.OneTimeTokens
}

/*
Handle issues the link and queues the mail, inside the transaction in
the context when there is one.
*/
func (a *SendEmailVerification) Handle(ctx context.Context, user models.User, client appauth.Client) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		issued, err := a.Tokens.Issue(ctx, appauth.Issue{
			Purpose: enums.OneTimePurposeEmailVerification, UserID: &user.ID, Email: user.Email,
			Client: client, TTL: verificationTTL, Live: verificationLive,
		})
		if err != nil {
			return err
		}

		return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.VerifyEmailMail{
			Name: user.Name, Email: user.Email, URL: a.Links.VerifyEmail(issued.ID, issued.Token),
		})
	})
}
