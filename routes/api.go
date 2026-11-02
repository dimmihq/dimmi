/*
|--------------------------------------------------------------------------
| API Routes
|--------------------------------------------------------------------------
|
| The public REST API under /api/v1. Routes are grouped by who may call
| them, and every group that a browser can reach with its cookie runs
| cross-origin protection on unsafe requests, signing in and signing up
| included, so no other site can sign a browser into an account it
| chose.
|
|   guest   anyone; an existing session is allowed and ignored
|   user    the user themselves, by browser or by their own app
|   caller  every valid credential, access tokens included
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
API declares the /api/v1 routes.
*/
func API(a *app.App) {
	server := v1.Server{
		Edition: string(a.Config.App.Edition), Version: a.Version, APIVersion: a.Config.API.Version,
		Region: a.Config.App.Region, MailConfigured: a.MailConfigured(), Clock: a.Clock,
	}

	authConfig := &v1.AuthConfigController{Auth: a.Config.Auth, Server: server}
	users := &v1.UserController{RegisterUser: a.Actions.RegisterUser, Cookies: a.Cookies, Server: server}
	me := &v1.MeController{LoadMe: a.Actions.LoadMe, IDs: a.IDs, Server: server}
	sessions := &v1.SessionController{
		AttemptLogin: a.Actions.AttemptLogin, ListSessions: a.Actions.ListSessions, EndSession: a.Actions.EndSession,
		EndOtherSessions: a.Actions.EndOtherSessions, IDs: a.IDs, Cookies: a.Cookies, Server: server,
	}

	a.Router.API("/api/v1", "Dimmi API", a.Config.API.Version, func(api *routing.Router) {
		api.Group("", func(guest *routing.Router) {
			guest.Middleware("csrf", "session")

			routing.Get(guest, "/auth/config", "auth.config.show", authConfig.Show)
			routing.Post(guest, "/users", "users.store", users.Store)
			routing.Post(guest, "/sessions", "sessions.store", sessions.Store)
		})

		api.Group("", func(user *routing.Router) {
			user.Middleware("csrf", "session", "auth:user")

			routing.Get(user, "/sessions", "sessions.index", sessions.Index)
			routing.Delete(user, "/sessions", "sessions.destroy-many", sessions.DestroyMany).
				Middleware("password.confirm").Status(fhttp.StatusNoContent)
			routing.Delete(user, "/sessions/{session}", "sessions.destroy", sessions.Destroy).
				Status(fhttp.StatusNoContent)
		})

		api.Group("", func(caller *routing.Router) {
			caller.Middleware("csrf", "session", "auth")

			routing.Get(caller, "/me", "me.show", me.Show)
		})
	})
}
