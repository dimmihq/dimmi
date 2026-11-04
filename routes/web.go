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
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
	"github.com/dimmihq/dimmi/app/http/controllers/widget"
	"github.com/dimmihq/dimmi/app/support/helpsample"
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

		helpCenter(pages, a)
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
helpCenter declares the public portal under the app's /hc/{center}
path: the help center, its changelog, feature requests, the visitor's
conversations, and the files their pages show.
*/
func helpCenter(pages *routing.Router, a *app.App) {
	hc := &web.HelpCenterController{
		Library: a.HelpSample, Errors: a.Errors, Assets: static.Handler(helpsample.Assets(), static.Options{}),
		Vite: vite.New(vite.Options{Build: a.Frontend, Base: "/build", HotFile: a.Config.Frontend.HotPath}),
	}
	cl := web.ChangelogController{HelpCenterController: hc}
	get := func(path, name string, h fhttp.HandlerFunc) {
		pages.Handle(fhttp.MethodGet, "/hc/{center}"+path, name, h)
	}
	post := func(path, name string, h fhttp.HandlerFunc) {
		pages.Handle(fhttp.MethodPost, "/hc/{center}"+path, name, h)
	}

	get("", "hc.home", hc.Home)
	get("/collections/{slug}", "hc.collections.show", hc.Collection)
	get("/articles/{slug}", "hc.articles.show", hc.Article)
	post("/articles/{slug}/feedback", "hc.articles.feedback", hc.Rate)
	get("/search", "hc.search", hc.Search)
	get("/changelog", "hc.changelog.index", cl.Index)
	get("/changelog/page/{page}", "hc.changelog.index.page", cl.Index)
	get("/changelog/labels/{label}", "hc.changelog.labels.show", cl.Index)
	get("/changelog/labels/{label}/page/{page}", "hc.changelog.labels.page", cl.Index)
	get("/changelog/{slug}", "hc.changelog.entries.show", cl.Entry)
	post("/changelog/subscriptions", "hc.changelog.subscriptions.store", cl.Subscribe)
	post("/changelog/{slug}/reactions", "hc.changelog.entries.reactions.store", cl.React)
	portalPages(get, post, web.PortalController{HelpCenterController: hc})
	get("/assets/{file}", "hc.assets", hc.Asset)
	get("/*", "hc.missing", hc.NotFound)
}

/*
portalPages declares the portal's pages beside the help center and
changelog: feature requests, sign-in and the visitor's conversations.
*/
func portalPages(get, post func(path, name string, h fhttp.HandlerFunc), portal web.PortalController) {
	get("/feedback", "hc.feedback.index", portal.Feedback)
	get("/feedback/new", "hc.feedback.create", portal.NewIdea)
	post("/feedback", "hc.feedback.store", portal.SubmitIdea)
	post("/feedback/{idea}/votes", "hc.feedback.votes.store", portal.Vote)
	get("/sign-in", "hc.sign-in", portal.SignIn)
	post("/sign-in", "hc.sessions.store", portal.StartSession)
	post("/sign-out", "hc.sessions.destroy", portal.EndSession)
	get("/conversations", "hc.conversations.index", portal.Conversations)
	get("/conversations/new", "hc.conversations.create", portal.NewConversation)
	post("/conversations", "hc.conversations.store", portal.StartConversation)
	get("/conversations/{conversation}", "hc.conversations.show", portal.Conversation)
	post("/conversations/{conversation}/messages", "hc.conversations.messages.store", portal.Reply)
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
