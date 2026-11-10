/*
|--------------------------------------------------------------------------
| OAuth Routes
|--------------------------------------------------------------------------
|
| Agents and their apps sign in through OAuth. The server's own endpoints
| live at the application's root: its metadata, the token, revocation
| and registration endpoints. Protected resource metadata tells an MCP
| client that finds /mcp locked where to sign in. The person approves
| or refuses an app on the authorization page, backed by the consent API
| on their own session, and sees and disconnects their apps from the API
| too.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/oauthapps"
	appauth "github.com/dimmihq/dimmi/app/auth"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
mcpResourcePath is where the protected resource metadata of /mcp lives.
*/
const mcpResourcePath = "/.well-known/oauth-protected-resource/mcp"

/*
OAuth declares the OAuth server's endpoints and the protected resource
metadata.
*/
func OAuth(a *app.App) {
	a.Authorizer.Routes(a.Router)

	resource := mcp.ProtectedResourceHandler(mcp.ResourceMetadata{
		Resource: a.Application.Config.App.URL + "/mcp", AuthorizationServer: a.Authorizer.Issuer(),
		Scopes: appauth.OAuthScopes(), Name: "dimmi",
	})
	a.Router.Handle(fhttp.MethodGet, mcpResourcePath, "oauth.resource.mcp", resource)
	a.Router.Handle(fhttp.MethodGet, "/.well-known/oauth-protected-resource", "oauth.resource", resource)
}

/*
mcpResourceURL is the protected resource metadata URL /mcp names in its
challenges.
*/
func mcpResourceURL(a *app.App) string {
	return a.Application.Config.App.URL + mcpResourcePath
}

/*
oauthAppRoutes declares the consent API and the person's connected
apps, on their own session.
*/
func oauthAppRoutes(api *routing.Router, a *app.App) {
	c := &v1.OAuthAppController{
		Authorize: &oauthapps.Authorize{DB: a.DB, Server: a.Authorizer},
		Connected: &oauthapps.Connected{DB: a.DB, Server: a.Authorizer},
		IDs:       a.IDs,
	}

	api.Group("", func(user *routing.Router) {
		user.Middleware("csrf", "session", "auth:user", "api.throttle")

		routing.Get(user, "/oauth/authorization", "oauth.authorization.show", c.Show)
		routing.Post(user, "/oauth/authorizations", "oauth.authorizations.store", c.Store).Status(fhttp.StatusOK)
		routing.Get(user, "/user/connected-apps", "user.connected-apps.index", c.Index)
		routing.Delete(user, "/user/connected-apps/{app}", "user.connected-apps.destroy", c.Destroy).
			Status(fhttp.StatusNoContent)
	})
}
