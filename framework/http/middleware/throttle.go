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
requests are allowed per Decay. Limit, when set, works out the number
for each request instead, and a number of zero or less means no limit.
Key picks the counter for a request; without it the client's address is
used, and an empty key leaves the request unlimited. Skip lets a request
through uncounted. Scope names the limit in the X-RateLimit-Scope
header, so a client can tell which of several limits answered. FailOpen
lets requests through while the counters cannot be reached. Clock dates
the reset header; without it the system clock is used.
*/
type ThrottleOptions struct {
	Name        string
	MaxAttempts int
	Decay       time.Duration
	Limit       func(r *http.Request) int
	Key         func(r *http.Request) string
	Skip        func(r *http.Request) bool
	Scope       string
	FailOpen    bool
	Clock       clock.Clock
}

/*
ScopeHeader names the limit that answered a limited response.
*/
const ScopeHeader = "X-RateLimit-Scope"

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
			key, limit, counted := opts.counter(r)
			if !counted {
				next.ServeHTTP(w, r)

				return
			}

			res, err := l.Attempt(r.Context(), opts.Name+":"+key, limit, opts.Decay)
			if err != nil {
				unchecked(r, err, opts)(next).ServeHTTP(w, r)

				return
			}

			res.WriteHeaders(w.Header(), opts.Clock.Now())
			if opts.Scope != "" {
				w.Header().Set(ScopeHeader, opts.Scope)
			}
			if !res.Allowed {
				refuse(w, r, fault.New(fault.RateLimited, "too_many_requests", "Too many requests."))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

/*
counter returns the key and the number of requests allowed for the
request, and whether it is counted at all.
*/
func (opts ThrottleOptions) counter(r *http.Request) (string, int, bool) {
	if opts.Skip != nil && opts.Skip(r) {
		return "", 0, false
	}

	limit := opts.MaxAttempts
	if opts.Limit != nil {
		limit = opts.Limit(r)
	}

	if limit <= 0 {
		return "", 0, false
	}

	key := opts.Key(r)

	return key, limit, key != ""
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
