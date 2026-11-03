/*
|--------------------------------------------------------------------------
| Content Security Policy
|--------------------------------------------------------------------------
|
| Pages run only the scripts this server names. Every script tag carries
| a fresh nonce, styles and fonts come from this origin, and nothing may
| frame a page or post its forms elsewhere. The main domain hosts the
| app and its sessions, so no third-party script is ever allowed: one
| would act as the signed-in user.
|
| While the Vite dev server runs, pages also load modules, styles and
| fonts from it and keep a socket open for hot updates, and the styles
| it injects inline are allowed. A production server never runs one, so
| it never relaxes the policy.
|
*/

package middleware

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
)

/*
DevServer reports the address of the running Vite dev server, if any.
*/
type DevServer interface {
	/*
		DevServer returns the dev server's address, and false when none
		runs.
	*/
	DevServer() (string, bool)
}

/*
ContentSecurityPolicy returns the middleware that sets the security
headers of pages, the policy relaxed for the dev server while it runs.
*/
func ContentSecurityPolicy(dev DevServer) fhttp.Middleware {
	production := middleware.SecurityHeaders(middleware.SecurityHeadersOptions{ContentSecurityPolicy: policy("")})

	return func(next fhttp.Handler) fhttp.Handler {
		strict := production(next)

		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			server, running := dev.DevServer()
			if !running {
				strict.ServeHTTP(w, r)

				return
			}

			relaxed := middleware.SecurityHeadersOptions{ContentSecurityPolicy: policy(server)}
			middleware.SecurityHeaders(relaxed)(next).ServeHTTP(w, r)
		})
	}
}

/*
policy returns the policy, allowing the dev server at the address when
one is given.
*/
func policy(dev string) string {
	script, style, connect, assets := "", "", "", ""
	if dev != "" {
		socket := "ws" + strings.TrimPrefix(dev, "http")
		script, style, connect, assets = " "+dev, " 'unsafe-inline' "+dev, " "+dev+" "+socket, " "+dev
	}

	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'nonce-" + middleware.NoncePlaceholder + "'" + script,
		"style-src 'self'" + style,
		"img-src 'self' data: blob:" + assets,
		"font-src 'self'" + assets,
		"connect-src 'self'" + connect,
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}
