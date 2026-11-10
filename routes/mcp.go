/*
|--------------------------------------------------------------------------
| MCP Route
|--------------------------------------------------------------------------
|
| AI agents reach dimmi at /mcp with an OAuth access token from signing
| in, or a personal access token: the credentials the REST API takes.
| The server behind it offers every API operation the token allows as a
| tool and runs each call through the API itself, so an agent can do
| exactly what the token's owner could through the API, and nothing more. Only the application's own origin
| may call it from a browser.
|
*/

package routes

import (
	"net/http"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/mcp"

	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/mcpserver"
)

/*
mcpMaxBody caps one MCP message, generous for any tool's arguments.
*/
const mcpMaxBody = 1 << 20

/*
MCP declares the MCP endpoint.
*/
func MCP(a *app.App) {
	tokens := fauth.BearerGuard(
		&appauth.TokenGuard{DB: a.DB, Tokens: a.Tokens},
		&appauth.OAuthGuard{DB: a.DB, Server: a.Authorizer.Resolver()},
	)
	build := func() (*mcp.Server, error) {
		return mcpserver.New(mcpserver.Options{
			Router: a.Router, Sealer: a.Encrypter, Clock: a.Clock, Errors: a.Errors, Version: a.Version,
			API: func() (http.Handler, error) {
				routes, err := a.Router.Handler()
				if err != nil {
					return nil, err
				}

				return a.Kernel.Handler(routes), nil
			},
		})
	}

	handler := mcpserver.Handler(build, mcp.Options{
		MaxBody: mcpMaxBody, Auth: tokens, AllowedOrigins: []string{a.Application.Config.App.URL},
		ResourceMetadata: mcpResourceURL(a),
	}, a.Errors)

	a.Router.Handle(http.MethodPost, "/mcp", "mcp.store", handler)
}
