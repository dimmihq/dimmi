/*
|--------------------------------------------------------------------------
| Resend Email Verification
|--------------------------------------------------------------------------
|
| A signed-in user who lost the verification mail asks for another. An
| address already proven needs none, and asking is throttled, so the
| button cannot be used to flood an inbox.
|
*/

package auth

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
ResendEmailVerification mails the caller a new verification link.

Rules:
  - A verified user gets no mail.
  - One request a minute and six an hour per user.

Errors: [exceptions.ErrTooManyAttempts], [exceptions.ErrAuthBusy], or
those of [SendEmailVerification].
*/
type ResendEmailVerification struct {
	Limiter *ratelimit.Limiter
	Send    *SendEmailVerification
}

/*
Handle sends the link and reports whether it did, false for a user
already verified.
*/
func (a *ResendEmailVerification) Handle(
	ctx context.Context, p *appauth.Principal, client appauth.Client,
) (bool, error) {
	if p.User.IsVerified() {
		return false, nil
	}

	user := p.User.ID.String()

	err := appauth.Throttle(ctx, a.Limiter,
		appauth.Rate{Key: "verification:minute:" + user, Limit: 1, Window: time.Minute},
		appauth.Rate{Key: "verification:hour:" + user, Limit: 6, Window: time.Hour},
	)
	if err != nil {
		return false, err
	}

	return true, a.Send.Handle(ctx, p.User, client)
}
