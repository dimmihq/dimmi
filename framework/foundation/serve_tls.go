/*
|--------------------------------------------------------------------------
| Plain HTTP Beside HTTPS
|--------------------------------------------------------------------------
|
| When the application terminates TLS, plain HTTP exists for three
| things only: health checks a load balancer makes without TLS, the
| certificate authority's HTTP challenges, and sending every visitor to
| HTTPS. Nothing else is ever served unencrypted.
|
*/

package foundation

import (
	"errors"
	"net"
	"net/http"
	"net/url"

	"github.com/dimmihq/dimmi/framework/customdomain"
)

/*
ErrNoTLSDriver stops the boot when an HTTPS address is configured but the
default custom domain driver does not terminate TLS.
*/
var ErrNoTLSDriver = errors.New(
	"foundation: http.tls_addr is set but the default custom_domains driver provides no TLS configuration",
)

/*
redirectHandler serves the plain listener in TLS mode: health as on
every listener, challenges first, and a permanent redirect to the same
host and path over HTTPS for anything else. The redirect names no port,
because visitors reach HTTPS on 443 whatever port the process binds. A
Host header that is not a valid host name gets a 400 rather than a
redirect built from it.
*/
func (a *Application) redirectHandler() http.Handler {
	mux := a.healthMux()
	mux.Handle("/", http.HandlerFunc(redirectToHTTPS))

	return a.Domains.HTTPHandler(mux)
}

func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	name := r.Host
	if bare, _, err := net.SplitHostPort(name); err == nil {
		name = bare
	}

	host, err := customdomain.Normalize(name)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}

	target := url.URL{Scheme: "https", Host: host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}
	w.Header().Set("Location", target.String())
	w.WriteHeader(http.StatusPermanentRedirect)
}
