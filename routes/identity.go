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
	identitySecretRoutes(api, a)
}

/*
identitySecretRoutes declares the routes of the secrets a workspace's
server signs visitors' identity tokens with. Tokens need settings:read
to list them and settings:write to change them; the actions then check
the member may manage identity. Creating and rotating take no
idempotency key, since a replay cache would keep the secret in plain
text.
*/
func identitySecretRoutes(api *routing.Router, a *app.App) {
	if a.Actions.IdentitySecrets == nil {
		return
	}
	c := &v1.IdentitySecretController{Secrets: a.Actions.IdentitySecrets, IDs: a.IDs}
	one := "/identity-secrets/{secret}"

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

		routing.Get(r, "/identity-secrets", "identity-secrets.index", c.Index).Ability("settings:read")
		routing.Post(r, "/identity-secrets", "identity-secrets.store", c.Store).Ability("settings:write").
			Status(fhttp.StatusCreated)
		routing.Get(r, one, "identity-secrets.show", c.Show).Ability("settings:read")
		routing.Patch(r, one, "identity-secrets.update", c.Update).Ability("settings:write")
		routing.Post(r, one+"/rotations", "identity-secrets.rotations.store", c.Rotate).
			Ability("settings:write").Status(fhttp.StatusCreated)
	})

	identityVerificationRoutes(api, a)
}

/*
identityVerificationRoutes declares the routes of visitor sign-in
settings, counts and the token checker. Changing the mode takes the
person themselves, having just proven who they are: required can lock
every customer out, and off stops trusting signed-in history. A pasted
token is checked with no idempotency key, so it is never kept.
*/
func identityVerificationRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions
	c := &v1.IdentityVerificationController{
		Setup: acts.IdentitySetup, Stats: acts.IdentityStats, Checker: acts.IdentityChecker,
		Secrets: acts.IdentitySecrets, IDs: a.IDs,
	}

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

		routing.Get(r, "/identity-verification/settings", "identity-verification.settings.show", c.ShowSettings).
			Ability("settings:read")
		routing.Get(r, "/identity-verification/stats", "identity-verification.stats.show", c.ShowStats).
			Ability("settings:read")
		routing.Post(r, "/identity-verification/validations", "identity-verification.validations.store", c.Check).
			Ability("settings:read").Status(fhttp.StatusOK)
		routing.Delete(r, "/contacts/{contact}/widget-sessions", "contacts.widget-sessions.destroy", c.SignOut).
			Ability("settings:write").Status(fhttp.StatusOK)
	})

	api.Group("", func(confirmed *routing.Router) {
		confirmed.Middleware(
			"csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle", "password.confirm",
		)

		routing.Patch(confirmed, "/identity-verification/settings", "identity-verification.settings.update",
			c.UpdateSettings)
	})
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
	tests := &v1.SSOConnectionTestController{Start: acts.StartSSOTest, IDs: a.IDs, Cookies: a.Cookies}

	lookups := &v1.SSOLookupController{Lookup: acts.LookupSSO, Cookies: a.Cookies}

	api.Group("", func(guest *routing.Router) {
		guest.Middleware("csrf", "session")

		routing.Post(guest, "/sso-lookups", "sso-lookups.store", lookups.Store).Status(fhttp.StatusOK)
	})

	api.Group("", func(owners *routing.Router) {
		owners.Middleware("csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle")

		routing.Get(owners, "/sso-connections", "sso-connections.index", connections.Index)
		routing.Get(owners, "/sso-connections/{connection}", "sso-connections.show", connections.ShowConnection)
		routing.Get(owners, "/organization-domains", "organization-domains.index", domains.Index)
		routing.Post(owners, "/organization-domains/{domain}/checks", "organization-domains.checks.store",
			domains.Check).Status(fhttp.StatusCreated)
		routing.Post(owners, "/sso-connections/{connection}/tests", "sso-connections.tests.store", tests.Store).
			Status(fhttp.StatusCreated)
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
