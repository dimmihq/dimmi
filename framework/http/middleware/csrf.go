/*
|--------------------------------------------------------------------------
| Cross-Origin Protection
|--------------------------------------------------------------------------
|
| A browser sends cookies with requests that other sites trigger, so an
| unsafe request riding on a session cookie must come from the site
| itself. Browsers say where a request comes from in Sec-Fetch-Site, or
| failing that in Origin, and a request from elsewhere is refused. No
| token is needed. A request with an Authorization header is not riding
| on ambient credentials, since no other site can make a browser add
| one, so it passes.
|
*/

package middleware

import (
	"fmt"
	"net/http"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
CSRF refuses unsafe cross-origin requests that rely on cookies, with 403
cross_origin_request. Trusted origins, such as a dashboard served from
another host, are let through.
*/
func CSRF(trustedOrigins ...string) (fhttp.Middleware, error) {
	protection := http.NewCrossOriginProtection()
	for _, o := range trustedOrigins {
		if err := protection.AddTrustedOrigin(o); err != nil {
			return nil, fmt.Errorf("trust origin %q: %w", o, err)
		}
	}

	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refuse(w, r, fault.New(fault.Forbidden, "cross_origin_request", "Cross-origin request refused."))
	}))

	return func(next http.Handler) http.Handler {
		protected := protection.Handler(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				next.ServeHTTP(w, r)

				return
			}

			protected.ServeHTTP(w, r)
		})
	}, nil
}
