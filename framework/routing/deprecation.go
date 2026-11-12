/*
|--------------------------------------------------------------------------
| Deprecated Routes
|--------------------------------------------------------------------------
|
| A route on its way out stays in the API until its sunset, at least a
| year after it was deprecated. Until then every response tells the
| caller, through the Deprecation (RFC 9745) and Sunset (RFC 8594)
| headers, and the API document marks the operation deprecated, so SDKs
| generated from it warn their users at compile time.
|
*/

package routing

import (
	stdhttp "net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
Deprecation says when a route was deprecated, when it stops answering,
and where to read about its replacement.
*/
type Deprecation struct {
	Since  time.Time
	Sunset time.Time
	Link   string
}

/*
Deprecated marks the route deprecated: the API document flags the
operation and every response carries the Deprecation, Sunset and Link
headers.
*/
func (r *Route) Deprecated(d Deprecation) *Route {
	r.deprecation = &d

	return r.Use(deprecationHeaders(d))
}

/*
deprecationHeaders sets the deprecation headers before the handler
writes its response.
*/
func deprecationHeaders(d Deprecation) http.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			h := w.Header()
			h.Set("Deprecation", "@"+strconv.FormatInt(d.Since.Unix(), 10))

			if !d.Sunset.IsZero() {
				h.Set("Sunset", d.Sunset.UTC().Format(stdhttp.TimeFormat))
			}

			if d.Link != "" {
				h.Add("Link", "<"+d.Link+`>; rel="deprecation"; type="text/html"`)
			}

			next.ServeHTTP(w, req)
		})
	}
}

/*
deprecate flags a deprecated route's operation in the API document,
with its sunset date as x-sunset.
*/
func (r *Route) deprecate(op *huma.Operation) {
	if r.deprecation == nil {
		return
	}

	op.Deprecated = true

	if op.Extensions == nil {
		op.Extensions = map[string]any{}
	}

	if !r.deprecation.Sunset.IsZero() {
		op.Extensions["x-sunset"] = r.deprecation.Sunset.UTC().Format(time.DateOnly)
	}
}
