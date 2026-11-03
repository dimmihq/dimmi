/*
|--------------------------------------------------------------------------
| Throttles
|--------------------------------------------------------------------------
|
| Every endpoint that mails a secret or checks one counts its callers in
| Valkey, shared by every server: per address, per account, per secret.
| Some answer an exceeded limit openly with a wait; the ones that must
| not tell whether an account exists drop the request silently and
| answer as if it went through. Counters that cannot be read refuse, like
| sign-in does.
|
*/

package auth

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ratelimit"
)

/*
Rate is one limit: at most Limit calls per Window for the key.
*/
type Rate struct {
	Key    string
	Limit  int
	Window time.Duration
}

/*
Throttle counts the call against every rate and returns
exceptions.ErrTooManyAttempts, with how long to wait, for the first rate
it exceeds.
*/
func Throttle(ctx context.Context, limiter *ratelimit.Limiter, rates ...Rate) error {
	for _, rate := range rates {
		res, err := limiter.Attempt(ctx, rate.Key, rate.Limit, rate.Window)
		if err != nil {
			return Unavailable(ctx, err)
		}

		if !res.Allowed {
			return tooManyAttempts(res.RetryAfter)
		}
	}

	return nil
}

/*
Within counts the call against every rate and reports whether it stays
within all of them, for endpoints that drop excess calls silently.
*/
func Within(ctx context.Context, limiter *ratelimit.Limiter, rates ...Rate) (bool, error) {
	for _, rate := range rates {
		res, err := limiter.Attempt(ctx, rate.Key, rate.Limit, rate.Window)
		if err != nil {
			return false, Unavailable(ctx, err)
		}

		if !res.Allowed {
			return false, nil
		}
	}

	return true, nil
}
