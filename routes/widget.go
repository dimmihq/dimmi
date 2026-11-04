/*
|--------------------------------------------------------------------------
| Widget Routes
|--------------------------------------------------------------------------
|
| The API a visitor's browser calls, under /widget/v1, apart from the
| dashboard's API: it has its own contract, because SDKs in customers' apps
| depend on it, and its own credential. No cookie is ever read or set here.
| Public routes name their app and pass the origin check; everything else
| also carries the visitor's session.
|
*/

package routes

import (
	"context"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/http/useragent"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/widget"
	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
)

/*
Widget declares the /widget/v1 routes.
*/
func Widget(a *app.App) {
	install := installOrigin(a.Application.Config.App.URL)

	a.Router.Handle(fhttp.MethodOptions, "/widget/v1/*", "widget.preflight",
		appmiddleware.WidgetPreflight(a.Actions.Widget.Directory, a.IDs, install))

	a.Router.API("/widget/v1", "Dimmi Messenger API", a.Config.API.Version, func(api *routing.Router) {
		api.Middleware("widget.cors")

		api.Group("", func(public *routing.Router) {
			public.Middleware("widget.app", "widget.origin")
			widgetAppRoutes(public, a)
		})
		api.Group("", func(visitor *routing.Router) {
			visitor.Middleware("widget.session", "widget.origin")
			widgetSessionRoutes(visitor, a)
		})
	})
}

/*
widgetAppRoutes declares the routes that name their app.
*/
func widgetAppRoutes(public *routing.Router, a *app.App) {
	acts := a.Actions.Widget
	base := baseURLOf(a)

	config := &widget.ShowWidgetConfigController{Show: acts.ShowConfig, BaseURL: base}
	availability := &widget.ShowWidgetAvailabilityController{
		Compute: acts.Availability, Strings: acts.Strings, Clock: a.Clock,
	}

	routing.Get(public, "/apps/{app}/config", "widget.apps.config.show", config.Invoke)
	routing.Get(public, "/apps/{app}/availability", "widget.apps.availability.show", availability.Invoke)

	sessions := sessionController(a)
	routing.Post(public, "/sessions", "widget.sessions.store", sessions.Store)
}

/*
widgetSessionRoutes declares the routes a visitor's session names.
*/
func widgetSessionRoutes(visitor *routing.Router, a *app.App) {
	sessions := sessionController(a)

	routing.Get(visitor, "/session", "widget.session.show", sessions.Show)
	routing.Delete(visitor, "/session", "widget.session.destroy", sessions.Destroy).Status(fhttp.StatusNoContent)

	contact := &widget.ContactController{DB: a.DB, Update: a.Actions.Widget.UpdateContact}
	routing.Get(visitor, "/contact", "widget.contact.show", contact.Show)
	routing.Patch(visitor, "/contact", "widget.contact.update", contact.Patch)

	tokens := &widget.RealtimeTokenController{Realtime: a.Actions.Widget.Realtime, BaseURL: baseURLOf(a)}
	routing.Post(visitor, "/realtime-tokens", "widget.realtime-tokens.store", tokens.Store)

	pings := &widget.PingController{Record: a.Actions.Widget.Ping}
	routing.Post(visitor, "/pings", "widget.pings.store", pings.Store)

	conv := &widget.ConversationController{
		Read: a.Actions.Widget.Conversations, Post: a.Actions.Widget.PostMessage, IDs: a.IDs, Clock: a.Clock,
		Cursors: pagination.NewCursors(a.DerivedKey("widget.cursors")), Agents: useragent.New(),
	}
	one := "/conversations/{conversation}"

	routing.Get(visitor, "/conversations", "widget.conversations.index", conv.Index)
	routing.Post(visitor, "/conversations", "widget.conversations.store", conv.Store).Middleware("idempotent")
	routing.Get(visitor, one, "widget.conversations.show", conv.Show)
	routing.Get(visitor, one+"/messages", "widget.conversations.messages.index", conv.Messages)
	routing.Post(visitor, one+"/messages", "widget.conversations.messages.store", conv.Send).Middleware("idempotent")

	activity := &widget.ActivityController{Read: a.Actions.Widget.MarkRead, Tell: a.Actions.Widget.Typing, IDs: a.IDs}
	routing.Patch(visitor, one+"/read-state", "widget.conversations.read-state.update", activity.ReadState)
	routing.Post(visitor, one+"/typing-indicators", "widget.conversations.typing-indicators.store", activity.Typing)
}

/*
sessionController returns the controller of visitors' sessions.
*/
func sessionController(a *app.App) *widget.SessionController {
	acts := a.Actions.Widget

	return &widget.SessionController{
		Start: acts.StartSession, Describe: acts.ShowSession, Revoke: acts.RevokeSession, Clock: a.Clock,
		BaseURL: baseURLOf(a),
	}
}

/*
baseURLOf returns how to find the base address a request reached this install
on, which is the configured one until a custom domain says otherwise.
*/
func baseURLOf(a *app.App) func(ctx context.Context) string {
	base := strings.TrimRight(a.Application.Config.App.URL, "/")

	return func(context.Context) string { return base }
}

/*
installOrigin returns this install's own origin, the form browsers send.
*/
func installOrigin(appURL string) string {
	u, err := url.Parse(appURL)
	if err != nil {
		return ""
	}

	return u.Scheme + "://" + u.Host
}
