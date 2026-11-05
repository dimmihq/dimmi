/*
|--------------------------------------------------------------------------
| Captcha
|--------------------------------------------------------------------------
|
| Steps strangers can repeat for free, creating an account or mailing a
| sign-in link, can ask for proof that a person is there. A server with
| no provider asks nothing. A wrong answer is refused; a provider that
| is down refuses too, since letting everyone through while it is down
| is exactly what a script would wait for.
|
*/

package auth

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/support/turnstile"
)

/*
Captcha checks the answer a person at an address gave to a challenge.
*/
type Captcha interface {
	/*
		Verify returns nil for a genuine answer, turnstile.ErrRejected
		for a wrong one, and any other error when the provider could not
		tell.
	*/
	Verify(ctx context.Context, answer, ip string) error
}

/*
NoCaptcha asks nothing: every answer passes.
*/
type NoCaptcha struct{}

/*
Verify lets every answer pass.
*/
func (NoCaptcha) Verify(context.Context, string, string) error {
	return nil
}

/*
CheckCaptcha asks the captcha about the answer.

Errors: [exceptions.ErrCaptchaFailed] for a wrong or missing answer,
[exceptions.ErrCaptchaUnavailable] when the provider could not tell.
*/
func CheckCaptcha(ctx context.Context, c Captcha, answer, ip string) error {
	err := c.Verify(ctx, answer, ip)
	if err == nil {
		return nil
	}

	if errors.Is(err, turnstile.ErrRejected) {
		return exceptions.ErrCaptchaFailed
	}

	log.From(ctx).WarnContext(ctx, "captcha provider unavailable", log.Err(err))

	return exceptions.ErrCaptchaUnavailable
}
