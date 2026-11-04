/*
|--------------------------------------------------------------------------
| Widget Cross-Origin Rules
|--------------------------------------------------------------------------
|
| Any site may paste a snippet, so browsers must be allowed to read answers
| from the messenger's API. Every answer names the Origin that asked, never
| with credentials, since the messenger's credential is a header it sends
| itself and no cookie is involved. Whether that site may actually use a
| widget is decided by the origin check, which stops the request before any
| data is read; the refusal itself must stay readable, or the widget could
| not tell its developer why it does not work.
|
| Answers are never cached unless a route says so: a visitor's data must not
| outlive their session in anyone's cache.
|
*/

package middleware

import (
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/widgetorigin"
)

/*
ServerTimeHeader tells a visitor's browser the server's time, which it
offsets its own clock by.
*/
const ServerTimeHeader = "Dimmi-Server-Time"

/*
preflightMaxAge is how long a browser may remember a preflight, in seconds.
*/
const preflightMaxAge = "7200"

/*
WidgetCORS returns the middleware that marks answers readable by the asking
site, keeps them out of caches and tells the server's time. When the install
turned the messenger off, it answers every request with that instead.
*/
func WidgetCORS(enabled func() bool, clk clock.Clock, errs fhttp.Errors) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			reflectOrigin(w, r)

			h := w.Header()
			h.Set("Access-Control-Expose-Headers", "Dimmi-Server-Time, ETag, Retry-After, Idempotent-Replayed")
			h.Set("Cache-Control", "no-store")
			h.Set(ServerTimeHeader, clk.Now().UTC().Format(time.RFC3339))

			if !enabled() {
				errs.Render(w, r, exceptions.ErrWidgetUnavailable)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

/*
reflectOrigin names the asking site, when there is one.
*/
func reflectOrigin(w fhttp.ResponseWriter, r *fhttp.Request) {
	w.Header().Add("Vary", "Origin")

	if asking := r.Header.Get("Origin"); asking != "" {
		w.Header().Set("Access-Control-Allow-Origin", asking)
	}
}

/*
WidgetPreflight answers browsers asking whether they may call the messenger's
API. A route that names its app is checked against that widget's sites;
others cannot be, since a preflight carries no credential, and their real
request is checked instead.
*/
func WidgetPreflight(directory *widget.Directory, registry *ids.Registry, install string) fhttp.Handler {
	return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		w.Header().Add("Vary", "Origin")

		if !preflightAllowed(r, directory, registry, install) {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(fhttp.StatusForbidden)

			return
		}

		reflectOrigin(w, r)

		h := w.Header()
		h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE")
		h.Set("Access-Control-Allow-Headers",
			"Authorization, Content-Type, Idempotency-Key, Dimmi-Feedback-Session")
		h.Set("Access-Control-Max-Age", preflightMaxAge)
		w.WriteHeader(fhttp.StatusNoContent)
	})
}

/*
preflightAllowed checks the app the path names, if any. An app that does not
exist passes, so the real request can answer why.
*/
func preflightAllowed(
	r *fhttp.Request, directory *widget.Directory, registry *ids.Registry, install string,
) bool {
	_, rest, found := strings.Cut(r.URL.Path, "/apps/")
	if !found {
		return true
	}

	app, _, _ := strings.Cut(rest, "/")

	id, err := registry.Decode(string(models.PrefixWidget), app)
	if err != nil {
		return true
	}

	resolved, err := directory.Find(r.Context(), id)
	if err != nil {
		return true
	}

	return widgetorigin.OriginAllowed(
		resolved.Widget.AllowedOrigins, resolved.Widget.NativeAppsEnabled, r.Header.Get("Origin"), install,
	)
}
