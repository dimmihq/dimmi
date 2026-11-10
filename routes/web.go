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
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/view/static"
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
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

	a.Router.Group("", func(dashboard *routing.Router) {
		dashboard.Middleware("csp:dashboard")

		dashboard.Handle(fhttp.MethodGet, "/d", "app.show", fhttp.HandlerFunc(spa.Show))
		dashboard.Handle(fhttp.MethodGet, "/d/*", "app.page", fhttp.HandlerFunc(spa.Show))
	})

	a.Router.Group("", func(pages *routing.Router) {
		pages.Middleware("csp")

		pages.Handle(fhttp.MethodGet, "/", "home", fhttp.HandlerFunc(func(w fhttp.ResponseWriter, _ *fhttp.Request) {
			w.Header().Set("Location", "/d")
			w.WriteHeader(fhttp.StatusFound)
		}))

		for name, path := range authPages() {
			pages.Handle(fhttp.MethodGet, path, name, fhttp.HandlerFunc(spa.ShowAuth))
		}

		pages.Handle(fhttp.MethodGet, middleware.GatePath, "gate", fhttp.HandlerFunc(spa.ShowAuth))

		helpCenter(pages, a)
	})

	a.Router.Handle(fhttp.MethodGet, "/build/*", "build",
		static.Handler(a.Frontend, static.Options{Immutable: true, Prefix: "/build"}))

	browserFiles(a)
	broadcastLinks(a)

	a.Router.Group("", func(browser *routing.Router) {
		browser.Middleware("csp", "session")

		browser.Handle(fhttp.MethodGet, "/auth/{provider}/redirect", "oauth.redirect",
			fhttp.HandlerFunc(redirect.Invoke))
		browser.Handle(fhttp.MethodGet, "/auth/{provider}/callback", "oauth.callback",
			fhttp.HandlerFunc(callback.Invoke))
	})

	ssoPages(a)
}

/*
browserFiles declares the files browsers load outside the app's build:
the messenger's, and the dashboard's service worker.
*/
func browserFiles(a *app.App) {
	widgetAssets(a)
	a.Router.Handle(fhttp.MethodGet, "/sw.js", "service-worker", fhttp.HandlerFunc(web.ServiceWorker))
}

/*
widgetAssets declares the messenger's loader, its manifest and the files
it loads, which customers' sites fetch.
*/
func widgetAssets(a *app.App) {
	assets := &widget.AssetController{Assets: a.WidgetAssets, LoaderMaxAge: a.Config.Widget.LoaderMaxAge}
	a.Router.Handle(fhttp.MethodGet, "/widget/v1/loader.js", "widget.loader", fhttp.HandlerFunc(assets.Loader))
	a.Router.Handle(fhttp.MethodGet, "/widget/v1/manifest.json", "widget.manifest", fhttp.HandlerFunc(assets.Manifest))
	a.Router.Handle(fhttp.MethodGet, "/widget/assets/*", "widget.assets", fhttp.HandlerFunc(assets.Asset))
}

/*
ssoPages declares the pages a company's identity provider sends people
through. None takes the usual cross-site request check: the SAML answer
arrives as a post from the identity provider's own site, and the
attempt, its cookie and the request id the answer names stand in for it.
*/
func ssoPages(a *app.App) {
	acts := a.Actions
	c := &web.SSOController{
		Start: acts.StartSSOSignIn, SAML: acts.ConsumeSAML, OIDC: acts.ConsumeOIDC, Shows: acts.SSOMetadata, IDs: a.IDs,
		Cookies: a.Cookies, Errors: a.Errors,
	}

	a.Router.Group("", func(browser *routing.Router) {
		browser.Middleware("csp", "session")

		browser.Handle(fhttp.MethodGet, "/auth/sso/{connection}/redirect", "sso.redirect",
			fhttp.HandlerFunc(c.Redirect))
		browser.Handle(fhttp.MethodGet, "/auth/sso/{connection}/callback", "sso.callback",
			fhttp.HandlerFunc(c.Callback))
		browser.Handle(fhttp.MethodPost, "/auth/saml/{connection}/acs", "saml.acs", fhttp.HandlerFunc(c.ACS))
		browser.Handle(fhttp.MethodGet, "/auth/saml/{connection}/metadata", "saml.metadata",
			fhttp.HandlerFunc(c.Metadata))
	})
}

/*
helpCenter declares the public portal under the app's /hc/{center}
path: the help center, its changelog, feature requests, the visitor's
conversations, and the files their pages show.
*/
func helpCenter(pages *routing.Router, a *app.App) {
	hc := &web.HelpCenterController{
		Library: a.HelpSample, Public: a.Actions.HelpPublic, Readers: a.Actions.HelpReaders, Errors: a.Errors,
		Changelog: a.Actions.ChangelogPublic, Subscribers: a.Actions.ChangelogSubscribers, Board: a.Actions.Feedback,
		Assets: static.Handler(helpsample.Assets(), static.Options{}),
		Vite:   vite.New(vite.Options{Build: a.Frontend, Base: "/build", HotFile: a.Config.Frontend.HotPath}),
		AppURL: a.Application.Config.App.URL, Ratings: a.Actions.HelpRatings, Passes: a.Actions.IdentityPasses,
	}
	cl := web.ChangelogController{HelpCenterController: hc}
	images := &web.HelpImageController{Disks: a.Storage, Dir: helpcenters.ImageDir}
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
	post("/articles/{slug}/feedback/comment", "hc.articles.feedback.comment", hc.Comment)
	get("/search", "hc.search", hc.Search)
	get("/messenger-identity", "hc.messenger-identity", hc.MessengerIdentity)
	get("/sso", "hc.sso.show", hc.SignInPage)
	post("/sso", "hc.sso.store", hc.SignIn)
	post("/sso/sign-out", "hc.sso.destroy", hc.SignOut)
	get("/sso/sign-in", "hc.sso.sign-in", hc.EmailSignIn)
	post("/sso/codes", "hc.sso.codes.store", hc.SendCode)
	post("/sso/codes/check", "hc.sso.codes.check", hc.CheckCode)
	changelogPages(get, post, cl)
	get("/help-images/{file}", "hc.help-images.show", images.Show)
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
	get("/feedback/{idea}", "hc.feedback.show", portal.Idea)
	post("/feedback/{idea}/comments", "hc.feedback.comments.store", portal.Comment)
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
		"login":             "/login",
		"login.magic":       "/login/magic",
		"login.two-factor":  "/login/two-factor",
		"signup":            "/signup",
		"forgot-password":   "/forgot-password",
		"reset-password":    "/reset-password",
		"verify-email":      "/verify-email",
		"email.confirm":     "/email/confirm",
		"two-factor-reset":  "/two-factor-reset",
		"invitation":        "/invitation",
		"onboarding":        "/onboarding",
		"oauth.authorize":   "/oauth/authorize",
		"email.preferences": "/email-preferences",
	}
}

/*
changelogPages declares the changelog's public pages: its list, label
pages and feeds, each entry, and the forms and links readers use.
*/
func changelogPages(get, post func(path, name string, h fhttp.HandlerFunc), cl web.ChangelogController) {
	get("/changelog", "hc.changelog.index", cl.Index)
	get("/changelog/page/{page}", "hc.changelog.index.page", cl.Index)
	get("/changelog/labels/{label}", "hc.changelog.labels.show", cl.Index)
	get("/changelog/labels/{label}/page/{page}", "hc.changelog.labels.page", cl.Index)
	get("/changelog/feed.xml", "hc.changelog.feed", cl.Feed)
	get("/changelog/labels/{label}/feed.xml", "hc.changelog.labels.feed", cl.Feed)
	get("/changelog/{slug}", "hc.changelog.entries.show", cl.Entry)
	post("/changelog/subscriptions", "hc.changelog.subscriptions.store", cl.Subscribe)
	get("/changelog/subscribers/{subscriber}/{purpose}/{token}", "hc.changelog.subscribers.show", cl.SubscriberLink)
	post("/changelog/subscribers/{subscriber}/{purpose}/{token}", "hc.changelog.subscribers.update",
		cl.SubscriberAction)
	post("/changelog/{slug}/reactions", "hc.changelog.entries.reactions.store", cl.React)
}
