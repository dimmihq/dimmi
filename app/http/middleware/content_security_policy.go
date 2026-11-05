/*
|--------------------------------------------------------------------------
| Content Security Policy
|--------------------------------------------------------------------------
|
| Pages run only the scripts this server names. Every script tag carries
| a fresh nonce, fonts and style sheets come from this origin, and
| nothing may frame a page or post its forms elsewhere. The main domain
| hosts the app and its sessions, so no third-party script is allowed:
| one would act as the signed-in user. The one exception is a captcha
| the operator turns on: its provider's script and frame are then
| allowed, and the app loads them only on the signed-out pages that ask
| for an answer. With no captcha the policy names no other origin.
|
| Inline styles are allowed: the design system injects style elements
| at runtime, such as the reveal of a theme change, whose content varies
| so no hash can name it. Styles cannot run code, so scripts stay locked
| to the nonce.
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
Trusted lists the outside origins a page may load scripts and frames
from, such as a captcha provider's; empty in the strict policy.
*/
type Trusted struct {
	Scripts []string
	Frames  []string
}

/*
ContentSecurityPolicy returns the middleware that sets the security
headers of pages, allowing the trusted origins, and relaxed for the dev
server while it runs.
*/
func ContentSecurityPolicy(dev DevServer, trusted Trusted) fhttp.Middleware {
	production := middleware.SecurityHeaders(middleware.SecurityHeadersOptions{
		ContentSecurityPolicy: policy("", trusted),
	})

	return func(next fhttp.Handler) fhttp.Handler {
		strict := production(next)

		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			server, running := dev.DevServer()
			if !running {
				strict.ServeHTTP(w, r)

				return
			}

			relaxed := middleware.SecurityHeadersOptions{ContentSecurityPolicy: policy(server, trusted)}
			middleware.SecurityHeaders(relaxed)(next).ServeHTTP(w, r)
		})
	}
}

/*
policy returns the policy, allowing the dev server at the address when
one is given, and the trusted origins.
*/
func policy(dev string, trusted Trusted) string {
	script, style, connect, assets := "", "", "", ""
	if dev != "" {
		socket := "ws" + strings.TrimPrefix(dev, "http")
		script, style, connect, assets = " "+dev, " "+dev, " "+dev+" "+socket, " "+dev
	}

	script += sources(trusted.Scripts)
	frames := sources(trusted.Frames)

	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'nonce-" + middleware.NoncePlaceholder + "'" + script,
		"style-src 'self' 'unsafe-inline'" + style,
		"img-src 'self' data: blob:" + assets,
		"font-src 'self'" + assets,
		"connect-src 'self'" + connect,
		"frame-src 'self'" + frames,
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}

/*
sources writes the origins as the tail of a directive.
*/
func sources(origins []string) string {
	if len(origins) == 0 {
		return ""
	}

	return " " + strings.Join(origins, " ")
}
