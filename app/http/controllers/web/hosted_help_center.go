/*
|--------------------------------------------------------------------------
| Help Centers On Their Own Host
|--------------------------------------------------------------------------
|
| With a help domain, each help center answers at its own subdomain of
| it, and any help center may answer on a workspace's own verified
| domain. A request there is answered as the matching /hc/<subdomain>
| page, with every link starting at the host's root, so the same pages
| serve every address. The app's built files keep their own paths, and
| the routing probe answers on every host so a domain's check can reach
| it. A request on the app's own host is never looked up.
|
*/

package web

import (
	"context"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
)

/*
hostedKey marks a request answered on a help center's own host.
*/
type hostedKey struct{}

/*
CustomHosts names the subdomain of the help center answering on a
custom domain, empty when none does.
*/
type CustomHosts func(ctx context.Context, host string) string

/*
HostedHelpCenters serves help centers on their subdomains of the help
domain and on their custom domains.
*/
func HostedHelpCenters(addresses helpcenters.Addresses, custom CustomHosts) fhttp.Middleware {
	appHost := ""
	if u, err := url.Parse(addresses.AppURL); err == nil {
		appHost = strings.ToLower(u.Hostname())
	}
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			if keepsPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			sub := addresses.Subdomain(r.Host)
			if sub == "" && custom != nil && !ownHost(r.Host, appHost, addresses.BaseDomain) {
				sub = custom(r.Context(), r.Host)
			}
			if sub == "" {
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
keepsPath reports whether the path is served the same on every host.
*/
func keepsPath(path string) bool {
	return strings.HasPrefix(path, "/build/") || strings.HasPrefix(path, "/help-images/") ||
		strings.HasPrefix(path, helpcenters.DomainProbePath+"/")
}

/*
ownHost reports whether the host is the app's own, the help domain
itself, or plainly not a domain anyone could point here.
*/
func ownHost(host, appHost, baseDomain string) bool {
	host = strings.ToLower(host)
	if i := strings.LastIndexByte(host, ':'); i > 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	return host == "" || host == appHost || host == baseDomain || host == "localhost" ||
		!strings.Contains(host, ".") || strings.HasPrefix(host, "[") || isAddress(host)
}

/*
isAddress reports whether the host is an IPv4 address.
*/
func isAddress(host string) bool {
	return strings.Trim(host, "0123456789.") == ""
}

/*
hostedHere reports whether the request came in on a help center's own
host.
*/
func hostedHere(ctx context.Context) bool {
	hosted, _ := ctx.Value(hostedKey{}).(bool)
	return hosted
}
