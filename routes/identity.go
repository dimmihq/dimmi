/*
|--------------------------------------------------------------------------
| Identity Routes
|--------------------------------------------------------------------------
|
| How a company decides who gets into its workspace. Anyone who may read a
| workspace's settings may read its rules, access tokens included; changing
| them takes the person themselves, by browser or app, having just proven
| who they are. A stolen token must never be able to loosen a company's
| rules, and neither must an unattended laptop.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
identityRoutes declares the routes of the workspace's sign-in rules and
of its organization's single sign-on.
*/
func identityRoutes(api *routing.Router, a *app.App) {
	policy := &v1.AccessPolicyController{Show: a.Actions.ShowAccessPolicy, Update: a.Actions.UpdateAccessPolicy}

	api.Group("", func(readers *routing.Router) {
		readers.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

		routing.Get(readers, "/workspace/access-policy", "workspace.access-policy.show", policy.ShowPolicy).
			Ability("settings:read")
	})

	api.Group("", func(writers *routing.Router) {
		writers.Middleware(
			"csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle", "password.confirm",
		)

		routing.Patch(writers, "/workspace/access-policy", "workspace.access-policy.update", policy.UpdatePolicy)
	})

	ssoRoutes(api, a)
}

/*
ssoRoutes declares the routes of the organization's connections and
domains. Only the owners themselves reach them, never a token: a wrong
setting there locks everyone out.
*/
func ssoRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions
	connections := &v1.SSOConnectionController{
		List: acts.ListSSOConnections, Show: acts.ShowSSOConnection, Create: acts.CreateSSOConnection,
		Update: acts.UpdateSSOConnection, Delete: acts.DeleteSSOConnection, IDs: a.IDs, Links: a.Links,
	}
	domains := &v1.OrganizationDomainController{Domains: acts.OrganizationDomains, IDs: a.IDs}

	api.Group("", func(owners *routing.Router) {
		owners.Middleware("csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle")

		routing.Get(owners, "/sso-connections", "sso-connections.index", connections.Index)
		routing.Get(owners, "/sso-connections/{connection}", "sso-connections.show", connections.ShowConnection)
		routing.Get(owners, "/organization-domains", "organization-domains.index", domains.Index)
		routing.Post(owners, "/organization-domains/{domain}/checks", "organization-domains.checks.store",
			domains.Check).Status(fhttp.StatusCreated)
	})

	api.Group("", func(confirmed *routing.Router) {
		confirmed.Middleware(
			"csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle", "password.confirm",
		)

		routing.Post(confirmed, "/sso-connections", "sso-connections.store", connections.Store).
			Status(fhttp.StatusCreated)
		routing.Patch(confirmed, "/sso-connections/{connection}", "sso-connections.update",
			connections.UpdateConnection)
		routing.Delete(confirmed, "/sso-connections/{connection}", "sso-connections.destroy", connections.Destroy).
			Status(fhttp.StatusNoContent)
		routing.Post(confirmed, "/organization-domains", "organization-domains.store", domains.Store).
			Status(fhttp.StatusCreated)
		routing.Patch(confirmed, "/organization-domains/{domain}", "organization-domains.update", domains.Update)
		routing.Delete(confirmed, "/organization-domains/{domain}", "organization-domains.destroy", domains.Destroy).
			Status(fhttp.StatusNoContent)
	})
}
