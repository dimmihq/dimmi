/*
|--------------------------------------------------------------------------
| API Rate Limits
|--------------------------------------------------------------------------
|
| The public API shares the server fairly. Each access token has its own
| minute, cut into ten-second bursts so a script spreads its load, and
| all tokens of one workspace share a larger minute, so one runaway
| script cannot starve its teammates' others. A person in the browser or
| the app has a minute of their own and never counts against the
| workspace, so a script hitting its limit never slows the inbox.
|
| The numbers come from entitlements, so a customer who needs more gets
| an override; a catalog without them leaves the install's configured
| numbers, as does a lookup that fails. These are fairness limits: when the counters cannot be
| reached, requests go through. The limits run after sign-in and access
| checks, so a refused call costs nothing.
|
*/

package middleware

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/entitlements"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
burstWindow is the short window that spreads a token's minute, and
burstsPerMinute how many of them make a minute.
*/
const (
	burstWindow     = 10 * time.Second
	burstsPerMinute = 6
)

/*
APIThrottleOptions are the counters, where the numbers come from, the
numbers used when entitlements do not list them, the clock, and Refused,
called with the scope of every refused request, for metrics.
*/
type APIThrottleOptions struct {
	Limiter      middleware.Limiter
	Entitlements contracts.Entitlements
	Limits       appconfig.APILimitsConfig
	Clock        clock.Clock
	Refused      func(ctx context.Context, scope string)
}

/*
APIThrottle limits the API: the workspace's tokens together, then the
token's burst and minute, or the person's minute. The last limit to
pass sets the rate limit headers, so a client sees its own window; a
refusal carries the headers of the limit that refused it.
*/
func APIThrottle(o APIThrottleOptions) fhttp.Middleware {
	workspace := o.throttle(middleware.ThrottleOptions{
		Name: "api:workspace", Decay: time.Minute, Scope: "workspace", Key: workspaceKey,
		Limit: o.limit(entitlements.KeyAPIWorkspaceRateLimit, o.Limits.WorkspacePerMinute, 1),
	})
	burst := o.throttle(middleware.ThrottleOptions{
		Name: "api:burst", Decay: burstWindow, Scope: "burst", Key: tokenKey,
		Limit: o.limit(entitlements.KeyAPIRateLimit, o.Limits.TokenPerMinute, burstsPerMinute),
	})
	token := o.throttle(middleware.ThrottleOptions{
		Name: "api:token", Decay: time.Minute, Scope: "token", Key: tokenKey,
		Limit: o.limit(entitlements.KeyAPIRateLimit, o.Limits.TokenPerMinute, 1),
	})
	user := o.throttle(middleware.ThrottleOptions{
		Name: "api:user", Decay: time.Minute, Scope: "user", Key: userKey,
		Limit: o.limit(entitlements.KeyAPIUserRateLimit, o.Limits.UserPerMinute, 1),
	})

	return func(next fhttp.Handler) fhttp.Handler {
		chain := workspace(burst(token(user(next))))

		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			rec := &statusRecorder{ResponseWriter: w}
			chain.ServeHTTP(rec, r)

			if rec.status == fhttp.StatusTooManyRequests && o.Refused != nil {
				o.Refused(r.Context(), w.Header().Get(middleware.ScopeHeader))
			}
		})
	}
}

/*
throttle builds one fairness limit with the shared counters and clock.
*/
func (o APIThrottleOptions) throttle(opts middleware.ThrottleOptions) fhttp.Middleware {
	opts.FailOpen, opts.Clock = true, o.Clock

	return middleware.Throttle(o.Limiter, opts)
}

/*
limit returns how many requests the caller's workspace may make per
window under the key, its share of a minute when the minute is cut into
parts. A catalog without the key leaves the configured number.
*/
func (o APIThrottleOptions) limit(key string, configured, parts int64) func(r *fhttp.Request) int {
	return func(r *fhttp.Request) int {
		workspace := ids.Nil()
		if p, ok := appauth.CurrentPrincipal(r.Context()); ok {
			workspace = p.WorkspaceID
		}

		perMinute := entitlements.APILimit(r.Context(), o.Entitlements, entitlements.APILimitQuery{
			Workspace: workspace, Key: key, Configured: configured,
		})
		if perMinute <= 0 {
			return 0
		}

		return int(max(perMinute/parts, 1))
	}
}

/*
workspaceKey counts a token's request against its workspace; requests
from browsers and apps are left out.
*/
func workspaceKey(r *fhttp.Request) string {
	p, ok := appauth.CurrentPrincipal(r.Context())
	if !ok || p.Kind != appauth.KindToken {
		return ""
	}

	return "ws:" + p.WorkspaceID.String()
}

/*
tokenKey counts a token's request against the token.
*/
func tokenKey(r *fhttp.Request) string {
	p, ok := appauth.CurrentPrincipal(r.Context())
	if !ok || p.Kind != appauth.KindToken {
		return ""
	}

	return "tok:" + p.CredentialID.String()
}

/*
userKey counts a browser's or an app's request against the person.
*/
func userKey(r *fhttp.Request) string {
	p, ok := appauth.CurrentPrincipal(r.Context())
	if !ok || p.Kind == appauth.KindToken {
		return ""
	}

	return "usr:" + p.ID().String()
}

/*
statusRecorder remembers the status a handler answered with.
*/
type statusRecorder struct {
	fhttp.ResponseWriter
	status int
}

/*
WriteHeader records the status and sends it on.
*/
func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

/*
Unwrap gives the response writer underneath to http.ResponseController.
*/
func (s *statusRecorder) Unwrap() fhttp.ResponseWriter {
	return s.ResponseWriter
}
