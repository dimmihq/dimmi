/*
|--------------------------------------------------------------------------
| Content Security Policy
|--------------------------------------------------------------------------
|
| Pages run only the scripts this server names. Every script tag carries
| a fresh nonce, fonts and style sheets come from this origin, and
| nothing may frame a page or post its forms elsewhere. The main domain
| hosts the app and its sessions, so no third-party script is ever
| allowed: one would act as the signed-in user.
|
| Inline styles are allowed: the design system injects style elements
| at runtime, such as the reveal of a theme change, whose content varies
| so no hash can name it. Styles cannot run code, so scripts stay locked
| to the nonce.
|
| A page may name more sources, such as the dashboard reaching GIPHY
| when GIF search is on; only pages that need them get them.
|
| While the Vite dev server runs, pages also load modules, styles and
| fonts from it and keep a socket open for hot updates. A production
| server never runs one, so it never relaxes the policy further.
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
Sources are extra origins a page may reach: Connect for its requests and
Images for the pictures it shows.
*/
type Sources struct {
	Connect []string
	Images  []string
}

/*
ContentSecurityPolicy returns the middleware that sets the security
headers of pages allowing the extra sources, the policy relaxed for the
dev server while it runs.
*/
func ContentSecurityPolicy(dev DevServer, extra Sources) fhttp.Middleware {
	production := middleware.SecurityHeaders(middleware.SecurityHeadersOptions{
		ContentSecurityPolicy: policy("", extra),
	})

	return func(next fhttp.Handler) fhttp.Handler {
		strict := production(next)

		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			server, running := dev.DevServer()
			if !running {
				strict.ServeHTTP(w, r)

				return
			}

			relaxed := middleware.SecurityHeadersOptions{ContentSecurityPolicy: policy(server, extra)}
			middleware.SecurityHeaders(relaxed)(next).ServeHTTP(w, r)
		})
	}
}

/*
policy returns the policy with the extra sources, allowing the dev server
at the address when one is given.
*/
func policy(dev string, extra Sources) string {
	script, style, connect, assets := "", "", "", ""
	if dev != "" {
		socket := "ws" + strings.TrimPrefix(dev, "http")
		script, style, connect, assets = " "+dev, " "+dev, " "+dev+" "+socket, " "+dev
	}

	images := strings.Join(append([]string{assets}, extra.Images...), " ")
	connect = strings.Join(append([]string{connect}, extra.Connect...), " ")

	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'nonce-" + middleware.NoncePlaceholder + "'" + script,
		"style-src 'self' 'unsafe-inline'" + style,
		"img-src 'self' data: blob:" + images,
		"font-src 'self'" + assets,
		"connect-src 'self'" + connect,
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}
