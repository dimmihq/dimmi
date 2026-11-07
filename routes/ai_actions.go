/*
|--------------------------------------------------------------------------
| Agent Action Routes
|--------------------------------------------------------------------------
|
| The workspace's own actions for its agent: connections, actions and
| their versions, tests, and the calls the agent made. Tokens need
| ai:read to read, ai:actions to change connections and actions (they
| spend the workspace's secrets and act on its customers), and ai:write
| for a teammate to approve or decline a call; the actions then check
| the member's own permissions.
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
aiActionRoutes declares the agent action routes.
*/
func aiActionRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.AI
	if acts.ActionConnections == nil {
		return
	}
	aiActionConnectionRoutes(r, &v1.AIActionConnectionController{Connections: acts.ActionConnections, IDs: a.IDs})

	c := &v1.AIActionController{Definitions: acts.ActionDefinitions, IDs: a.IDs}
	one := "/ai/actions/{action}"
	routing.Get(r, "/ai/actions", "ai.actions.index", c.Index).Ability("ai:read")
	routing.Post(r, "/ai/actions", "ai.actions.store", c.Store).Ability("ai:actions").
		Middleware("idempotent").Status(fhttp.StatusCreated)
	routing.Get(r, one, "ai.actions.show", c.Show).Ability("ai:read")
	routing.Patch(r, one, "ai.actions.update", c.Update).Ability("ai:actions")
	routing.Delete(r, one, "ai.actions.destroy", c.Destroy).Ability("ai:actions").Status(fhttp.StatusNoContent)
	routing.Post(r, one+"/publications", "ai.actions.publications.store", c.Publish).Ability("ai:actions").
		Status(fhttp.StatusCreated)
	routing.Get(r, one+"/versions", "ai.actions.versions.index", c.Versions).Ability("ai:read")
	routing.Post(r, one+"/tests", "ai.actions.tests.store", c.Test).Ability("ai:actions").Status(fhttp.StatusCreated)

	calls := &v1.AIActionCallController{Calls: acts.ActionCalls, IDs: a.IDs}
	routing.Get(r, "/ai/action-calls", "ai.action-calls.index", calls.Index).Ability("ai:read")
	routing.Get(r, "/ai/action-calls/{call}", "ai.action-calls.show", calls.Show).Ability("ai:read")
	routing.Patch(r, "/ai/action-calls/{call}", "ai.action-calls.update", calls.Update).Ability("ai:write")
}

/*
aiActionConnectionRoutes declares the connection routes.
*/
func aiActionConnectionRoutes(r *routing.Router, c *v1.AIActionConnectionController) {
	one := "/ai/action-connections/{connection}"
	routing.Get(r, "/ai/action-connections", "ai.action-connections.index", c.Index).Ability("ai:read")
	routing.Post(r, "/ai/action-connections", "ai.action-connections.store", c.Store).Ability("ai:actions").
		Middleware("idempotent").Status(fhttp.StatusCreated)
	routing.Get(r, one, "ai.action-connections.show", c.Show).Ability("ai:read")
	routing.Patch(r, one, "ai.action-connections.update", c.Update).Ability("ai:actions")
	routing.Delete(r, one, "ai.action-connections.destroy", c.Destroy).Ability("ai:actions").
		Status(fhttp.StatusNoContent)
	routing.Post(r, one+"/tests", "ai.action-connections.tests.store", c.Test).Ability("ai:actions").
		Status(fhttp.StatusCreated)
	routing.Post(r, one+"/tool-syncs", "ai.action-connections.tool-syncs.store", c.SyncTools).
		Ability("ai:actions").Status(fhttp.StatusCreated)
}
