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
ErrCrossOrigin refuses a request a browser sent on another site's
behalf.
*/
var ErrCrossOrigin = fault.New(fault.Forbidden, "cross_origin_request", "Cross-origin request refused.")

/*
OriginCheck applies the cross-origin rules to requests that are not
unsafe by method but still act on the session cookie, such as the
upgrade that opens a realtime connection: a browser may open one to any
site, carrying that site's cookies.
*/
type OriginCheck struct {
	protection *http.CrossOriginProtection
}

/*
NewOriginCheck returns a check that also lets the trusted origins
through.
*/
func NewOriginCheck(trustedOrigins ...string) (*OriginCheck, error) {
	protection, err := crossOriginProtection(trustedOrigins)
	if err != nil {
		return nil, err
	}

	return &OriginCheck{protection: protection}, nil
}

/*
Check returns ErrCrossOrigin when a browser sent the request from
another origin, whatever its method. Requests from outside a browser
carry no origin and pass.
*/
func (c *OriginCheck) Check(r *http.Request) error {
	unsafe := r.Clone(r.Context())
	unsafe.Method = http.MethodPost

	if c.protection.Check(unsafe) != nil {
		return ErrCrossOrigin
	}

	return nil
}

/*
CSRF refuses unsafe cross-origin requests that rely on cookies, with 403
cross_origin_request. Trusted origins, such as a dashboard served from
another host, are let through.
*/
func CSRF(trustedOrigins ...string) (fhttp.Middleware, error) {
	protection, err := crossOriginProtection(trustedOrigins)
	if err != nil {
		return nil, err
	}

	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refuse(w, r, ErrCrossOrigin)
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

/*
crossOriginProtection returns the standard library's protection, also
trusting the origins.
*/
func crossOriginProtection(trustedOrigins []string) (*http.CrossOriginProtection, error) {
	protection := http.NewCrossOriginProtection()
	for _, o := range trustedOrigins {
		if err := protection.AddTrustedOrigin(o); err != nil {
			return nil, fmt.Errorf("trust origin %q: %w", o, err)
		}
	}

	return protection, nil
}
