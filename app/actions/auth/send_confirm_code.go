/*
|--------------------------------------------------------------------------
| Send Confirm Code
|--------------------------------------------------------------------------
|
| A user without a password confirms who they are with a six-digit code
| mailed to their address. Users with a password type it instead, so
| they get no code: a mailbox must never be a weaker second way in.
|
*/

package auth

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
)

/*
How long a confirmation code works, and how many may work at once.
*/
const (
	confirmCodeTTL  = 10 * time.Minute
	confirmCodeLive = 3
)

/*
SendConfirmCode mails the caller a code that confirms who they are.

Rules:
  - Only for users without a password.
  - One code a minute and five an hour per user.
  - The code works for ten minutes; five wrong tries burn it.

Errors: [exceptions.ErrMethodNotAvailable], [exceptions.ErrTooManyAttempts],
[exceptions.ErrAuthBusy], or a wrapped error.
*/
type SendConfirmCode struct {
	DB      *database.DB
	Mail    *mail.Manager
	Limiter *ratelimit.Limiter
	Tokens  *appauth.OneTimeTokens
}

/*
Handle issues the code and queues the mail.
*/
func (a *SendConfirmCode) Handle(ctx context.Context, p *appauth.Principal, client appauth.Client) error {
	user := p.User
	if user.HasPassword() {
		return exceptions.ErrMethodNotAvailable
	}

	err := appauth.Throttle(ctx, a.Limiter,
		appauth.Rate{Key: "confirm_code:minute:" + user.ID.String(), Limit: 1, Window: time.Minute},
		appauth.Rate{Key: "confirm_code:hour:" + user.ID.String(), Limit: 5, Window: time.Hour},
	)
	if err != nil {
		return err
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		issued, err := a.Tokens.Issue(ctx, appauth.Issue{
			Purpose: enums.OneTimePurposeConfirmCode, UserID: &user.ID, Email: user.Email, Client: client,
			TTL: confirmCodeTTL, WithCode: true, Live: confirmCodeLive,
		})
		if err != nil {
			return err
		}

		return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.ConfirmCodeMail{
			Name: user.Name, Code: appauth.FormatCode(issued.Code),
		})
	})
}
