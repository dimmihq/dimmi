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
