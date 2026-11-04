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
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
identityRoutes declares the routes of the workspace's sign-in rules.
*/
func identityRoutes(api *routing.Router, a *app.App) {
	policy := &v1.AccessPolicyController{Show: a.Actions.ShowAccessPolicy, Update: a.Actions.UpdateAccessPolicy}

	api.Group("", func(readers *routing.Router) {
		readers.Middleware("csrf", "session", "auth", "workspace", "access.policy")

		routing.Get(readers, "/workspace/access-policy", "workspace.access-policy.show", policy.ShowPolicy).
			Ability("settings:read")
	})

	api.Group("", func(writers *routing.Router) {
		writers.Middleware("csrf", "session", "auth:user", "workspace", "access.policy", "password.confirm")

		routing.Patch(writers, "/workspace/access-policy", "workspace.access-policy.update", policy.UpdatePolicy)
	})
}
