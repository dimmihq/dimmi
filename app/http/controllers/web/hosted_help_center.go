/*
|--------------------------------------------------------------------------
| Help Centers On Their Own Host
|--------------------------------------------------------------------------
|
| With a help domain, each help center answers at its own subdomain of
| it. A request there is answered as the matching /hc/<subdomain> page,
| with every link starting at the host's root, so the same pages serve
| both addresses. The app's built files keep their own paths.
|
*/

package web

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
)

/*
hostedKey marks a request answered on a help center's own host.
*/
type hostedKey struct{}

/*
HostedHelpCenters serves help centers on their subdomains of the help
domain.
*/
func HostedHelpCenters(addresses helpcenters.Addresses) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			sub := addresses.Subdomain(r.Host)
			if sub == "" || strings.HasPrefix(r.URL.Path, "/build/") || strings.HasPrefix(r.URL.Path, "/help-images/") {
				next.ServeHTTP(w, r)
				return
			}
			hosted := r.Clone(context.WithValue(r.Context(), hostedKey{}, true))
			hosted.URL.Path = strings.TrimSuffix("/hc/"+sub+r.URL.Path, "/")
			hosted.URL.RawPath = ""
			next.ServeHTTP(w, hosted)
		})
	}
}

/*
hostedHere reports whether the request came in on a help center's own
host.
*/
func hostedHere(ctx context.Context) bool {
	hosted, _ := ctx.Value(hostedKey{}).(bool)
	return hosted
}
