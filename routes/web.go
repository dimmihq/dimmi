/*
|--------------------------------------------------------------------------
| Web Routes
|--------------------------------------------------------------------------
|
| Pages a browser opens directly rather than calls from the dashboard:
| the browser app under /d, its sign-in pages and its built files under
| /build, the messenger widget's files customers' pages load, and the
| redirects to identity providers and their callbacks. Pages carry a
| strict content security policy. The provider routes run the browser's
| session, which a sign-in rotates and a link reads, but no guard: the
| redirect decides per attempt what it needs, and the callback trusts
| only the sealed cookie the redirect set.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/view/static"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
	"github.com/dimmihq/dimmi/app/http/controllers/widget"
)

/*
Web declares the browser page routes.
*/
func Web(a *app.App) {
	spa := &web.AppController{Vite: a.Vite, Errors: a.Errors}
	redirect := &web.OAuthRedirectController{
		Providers: a.Providers, Attempts: a.OAuthAttempts, Cookies: a.Cookies, Clock: a.Clock,
		ConfirmationTTL: a.Config.Auth.PasswordConfirmation.TTL, Errors: a.Errors,
	}
	callback := &web.OAuthCallbackController{
		Providers: a.Providers, Attempts: a.OAuthAttempts, Cookies: a.Cookies,
		Callback: a.Actions.HandleOAuthCallback, Clock: a.Clock, Errors: a.Errors,
	}

	a.Router.Group("", func(pages *routing.Router) {
		pages.Middleware("csp")

		pages.Handle(fhttp.MethodGet, "/", "home", fhttp.HandlerFunc(func(w fhttp.ResponseWriter, _ *fhttp.Request) {
			w.Header().Set("Location", "/d")
			w.WriteHeader(fhttp.StatusFound)
		}))
		pages.Handle(fhttp.MethodGet, "/d", "app.show", fhttp.HandlerFunc(spa.Show))
		pages.Handle(fhttp.MethodGet, "/d/*", "app.page", fhttp.HandlerFunc(spa.Show))

		for name, path := range authPages() {
			pages.Handle(fhttp.MethodGet, path, name, fhttp.HandlerFunc(spa.ShowAuth))
		}
	})

	a.Router.Handle(fhttp.MethodGet, "/build/*", "build",
		static.Handler(a.Frontend, static.Options{Immutable: true, Prefix: "/build"}))

	assets := &widget.AssetController{Assets: a.WidgetAssets, LoaderMaxAge: a.Config.Widget.LoaderMaxAge}
	a.Router.Handle(fhttp.MethodGet, "/widget/v1/loader.js", "widget.loader", fhttp.HandlerFunc(assets.Loader))
	a.Router.Handle(fhttp.MethodGet, "/widget/v1/manifest.json", "widget.manifest", fhttp.HandlerFunc(assets.Manifest))
	a.Router.Handle(fhttp.MethodGet, "/widget/assets/*", "widget.assets", fhttp.HandlerFunc(assets.Asset))

	a.Router.Group("", func(browser *routing.Router) {
		browser.Middleware("csp", "session")

		browser.Handle(fhttp.MethodGet, "/auth/{provider}/redirect", "oauth.redirect",
			fhttp.HandlerFunc(redirect.Invoke))
		browser.Handle(fhttp.MethodGet, "/auth/{provider}/callback", "oauth.callback",
			fhttp.HandlerFunc(callback.Invoke))
	})
}

/*
authPages names the browser app's sign-in pages, which mails and the
provider callbacks link to.
*/
func authPages() map[string]string {
	return map[string]string{
		"login":            "/login",
		"login.magic":      "/login/magic",
		"login.two-factor": "/login/two-factor",
		"signup":           "/signup",
		"forgot-password":  "/forgot-password",
		"reset-password":   "/reset-password",
		"verify-email":     "/verify-email",
		"invitation":       "/invitation",
	}
}
