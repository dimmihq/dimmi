/*
|--------------------------------------------------------------------------
| Role Routes
|--------------------------------------------------------------------------
|
| A workspace's custom roles and the permission catalog they are shaped
| from. Anyone who may see the members may read both, access tokens
| included; shaping roles takes the person themselves, by browser or app,
| having just proven who they are, since a role decides what everyone
| holding it may do.
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
roleRoutes declares the routes of the workspace's custom roles.
*/
func roleRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions
	c := &v1.RoleController{
		ListRoles: acts.ListRoles, ShowRole: acts.ShowRole, CreateRole: acts.CreateRole, UpdateRole: acts.UpdateRole,
		DeleteRole: acts.DeleteRole, ListPermissions: acts.ListPermissions, IDs: a.IDs,
	}

	api.Group("", func(readers *routing.Router) {
		readers.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

		routing.Get(readers, "/roles", "roles.index", c.Index).Ability("members:read")
		routing.Get(readers, "/roles/{role}", "roles.show", c.Show).Ability("members:read")
		routing.Get(readers, "/permissions", "permissions.index", c.Permissions).Ability("members:read")
	})

	api.Group("", func(writers *routing.Router) {
		writers.Middleware(
			"csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle", "password.confirm",
		)

		routing.Post(writers, "/roles", "roles.store", c.Store).Status(fhttp.StatusCreated)
		routing.Patch(writers, "/roles/{role}", "roles.update", c.Update)
		routing.Delete(writers, "/roles/{role}", "roles.destroy", c.Destroy).Status(fhttp.StatusNoContent)
	})
}
