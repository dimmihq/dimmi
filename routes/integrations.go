/*
|--------------------------------------------------------------------------
| Integration Routes
|--------------------------------------------------------------------------
|
| The tools a workspace connects. Reading them takes settings:read;
| disconnecting one takes settings:write, and the action checks
| integrations:manage. Installing a tool and connecting one's own
| account start from the person's own browser only, never a token, since
| the provider sends that same browser back to the callback, which
| counts only in the session that started. Installing also needs a
| verified email where the install requires one.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
)

/*
integrationRoutes declares the integrations' API routes.
*/
func integrationRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions.Integrations
	providers := &v1.IntegrationProviderController{List: acts.List}
	c := &v1.IntegrationController{List: acts.List, Disconnect: acts.Disconnect, IDs: a.IDs}

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

		routing.Get(r, "/integration-providers", "integration-providers.index", providers.Index).
			Ability("settings:read")
		routing.Get(r, "/integrations", "integrations.index", c.Index).Ability("settings:read").Query(
			routing.QueryParam{Name: "filter[provider]", Description: "Only installations of this provider."},
		)
		routing.Get(r, "/integrations/{integration}", "integrations.show", c.Show).Ability("settings:read")
		routing.Delete(r, "/integrations/{integration}", "integrations.destroy", c.Destroy).
			Ability("settings:write").Status(fhttp.StatusAccepted)
	})

	installs := &v1.IntegrationInstallationController{Start: acts.StartInstallation}
	grants := &v1.IntegrationMemberGrantController{
		Start: acts.StartMemberGrant, Revoke: acts.RevokeMemberGrant, IDs: a.IDs,
	}

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle")

		routing.Post(r, "/integrations/{provider}/installations", "integrations.installations.store",
			installs.Store).Middleware("verified").Status(fhttp.StatusOK)
		routing.Post(r, "/integrations/{integration}/member-grants", "integrations.member-grants.store",
			grants.Store).Status(fhttp.StatusOK)
		routing.Delete(r, "/integrations/{integration}/member-grants/{grant}",
			"integrations.member-grants.destroy", grants.Destroy).Status(fhttp.StatusNoContent)
	})
}

/*
Integrations declares the page providers send the browser back to.
*/
func Integrations(a *app.App) {
	callback := &web.IntegrationOAuthCallbackController{
		Complete: a.Actions.Integrations.CompleteCallback, Errors: a.Errors,
	}

	a.Router.Group("", func(browser *routing.Router) {
		browser.Middleware("csp", "session", "auth:user")

		browser.Handle(fhttp.MethodGet, "/integrations/{provider}/callback", "integrations.callback",
			fhttp.HandlerFunc(callback.Invoke))
	})
}
