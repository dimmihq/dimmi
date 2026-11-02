/*
|--------------------------------------------------------------------------
| Rate Limits
|--------------------------------------------------------------------------
|
| A limit counts requests per key, by default the client's address, and
| every limited response says how many are left so well-behaved clients
| slow down on their own. When the counter store is down each limit
| decides for itself: a guard against abuse fails closed and refuses,
| a fairness limit fails open and lets the request through.
|
*/

package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/ratelimit"
)

/*
Limiter counts attempts per key, as ratelimit.Limiter does.
*/
type Limiter interface {
	/*
		Attempt counts one attempt at the key unless the limit is reached.
	*/
	Attempt(ctx context.Context, key string, maxAttempts int, decay time.Duration) (ratelimit.Result, error)
}

/*
ThrottleOptions configure one limit.

Name keeps the limit's counters apart from other limits. MaxAttempts
requests are allowed per Decay. Key picks the counter for a request;
without it the client's address is used. FailOpen lets requests through
while the counters cannot be reached. Clock dates the reset header;
without it the system clock is used.
*/
type ThrottleOptions struct {
	Name        string
	MaxAttempts int
	Decay       time.Duration
	Key         func(r *http.Request) string
	FailOpen    bool
	Clock       clock.Clock
}

/*
Throttle limits requests, answering 429 too_many_requests with the rate
limit headers once the limit is reached.
*/
func Throttle(l Limiter, opts ThrottleOptions) fhttp.Middleware {
	if opts.Key == nil {
		opts.Key = fhttp.ClientIP
	}

	if opts.Clock == nil {
		opts.Clock = clock.System()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			res, err := l.Attempt(ctx, opts.Name+":"+opts.Key(r), opts.MaxAttempts, opts.Decay)
			if err != nil {
				unchecked(r, err, opts)(next).ServeHTTP(w, r)

				return
			}

			res.WriteHeaders(w.Header(), opts.Clock.Now())
			if !res.Allowed {
				refuse(w, r, fault.New(fault.RateLimited, "too_many_requests", "Too many requests."))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

/*
unchecked decides what happens to a request whose limit cannot be
checked: logged and let through when the limit fails open, refused with
503 otherwise.
*/
func unchecked(r *http.Request, err error, opts ThrottleOptions) fhttp.Middleware {
	ctx := r.Context()
	if opts.FailOpen {
		log.From(ctx).WarnContext(ctx, "rate limit skipped", log.Err(err))

		return func(next http.Handler) http.Handler { return next }
	}

	log.From(ctx).ErrorContext(ctx, "rate limit unavailable", log.Err(err))

	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			refuse(w, r, fault.New(fault.Unavailable, "service_unavailable", "Service temporarily unavailable."))
		})
	}
}
