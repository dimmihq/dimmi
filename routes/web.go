/*
|--------------------------------------------------------------------------
| Web Routes
|--------------------------------------------------------------------------
|
| Pages a browser opens directly rather than calls from the dashboard:
| the redirects to identity providers and their callbacks. They run the
| browser's session, which a sign-in rotates and a link reads, but no
| guard: the redirect decides per attempt what it needs, and the
| callback trusts only the sealed cookie the redirect set.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
)

/*
Web declares the browser page routes.
*/
func Web(a *app.App) {
	redirect := &web.OAuthRedirectController{
		Providers: a.Providers, Attempts: a.OAuthAttempts, Cookies: a.Cookies, Clock: a.Clock,
		ConfirmationTTL: a.Config.Auth.PasswordConfirmation.TTL, Errors: a.Errors,
	}
	callback := &web.OAuthCallbackController{
		Providers: a.Providers, Attempts: a.OAuthAttempts, Cookies: a.Cookies,
		Callback: a.Actions.HandleOAuthCallback, Errors: a.Errors,
	}

	a.Router.Group("", func(browser *routing.Router) {
		browser.Middleware("session")

		browser.Handle(fhttp.MethodGet, "/auth/{provider}/redirect", "oauth.redirect",
			fhttp.HandlerFunc(redirect.Invoke))
		browser.Handle(fhttp.MethodGet, "/auth/{provider}/callback", "oauth.callback",
			fhttp.HandlerFunc(callback.Invoke))
	})
}
