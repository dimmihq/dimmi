/*
|--------------------------------------------------------------------------
| Bad Credentials
|--------------------------------------------------------------------------
|
| An address that keeps sending tokens or session cookies that do not
| work is guessing, or broken; either way it is slowed. Only requests
| that carried a credential and were turned away count, so a browser
| that is simply signed out is never slowed. Like the other API limits
| this one fails open: with the counters out of reach, requests go on.
|
*/

package middleware

import (
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ratelimit"
)

/*
BadCredentialsOptions are the counters, how many turned-away requests an
address may send a minute, the session cookie's name, the clock that
dates the reset header, and how errors are written.
*/
type BadCredentialsOptions struct {
	Limiter   *ratelimit.Limiter
	PerMinute int
	Cookie    string
	Clock     clock.Clock
	Errors    fhttp.Errors
}

/*
SlowBadCredentials refuses an address that sent too many credentials
that did not work this minute, and counts each request that carried one
and was answered 401.
*/
func SlowBadCredentials(o BadCredentialsOptions) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			if o.PerMinute <= 0 || !carriesCredential(r, o.Cookie) {
				next.ServeHTTP(w, r)

				return
			}

			key := "api:bad-credentials:" + fhttp.ClientIP(r)

			wait, err := o.Limiter.AvailableIn(r.Context(), key, o.PerMinute, time.Minute)
			if err == nil && wait > 0 {
				ratelimit.Result{Limit: o.PerMinute, RetryAfter: wait}.WriteHeaders(w.Header(), o.Clock.Now())
				o.Errors.Render(w, r, fault.New(fault.RateLimited, "too_many_requests", "Too many requests."))

				return
			}

			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			if rec.status == fhttp.StatusUnauthorized {
				_, _ = o.Limiter.Hit(r.Context(), key, o.PerMinute, time.Minute)
			}
		})
	}
}

/*
carriesCredential reports whether the request sent a bearer token or a
session cookie.
*/
func carriesCredential(r *fhttp.Request, cookie string) bool {
	if r.Header.Get("Authorization") != "" {
		return true
	}

	_, err := r.Cookie(cookie)

	return err == nil
}
