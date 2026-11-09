/*
|--------------------------------------------------------------------------
| Operator Routes
|--------------------------------------------------------------------------
|
| Operator, the team's own agent, over the API: its settings, each
| teammate's threads and questions, briefs on a conversation, the
| changes it proposes and what it remembers. Tokens need
| conversations:read to talk with Operator and conversations:write to
| apply or revert its changes; changing its settings needs ai:write. The
| actions then check the member's own permissions, and every change
| Operator makes runs through the action a teammate would use.
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
operatorRoutes declares Operator's routes.
*/
func operatorRoutes(r *routing.Router, a *app.App) {
	r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")
	acts := a.Actions.OperatorAgent
	c := &v1.OperatorController{Settings: acts.Settings, Threads: acts.Threads, IDs: a.IDs}
	routing.Get(r, "/operator/settings", "operator.settings.show", c.ShowSettings).Ability("conversations:read")
	routing.Patch(r, "/operator/settings", "operator.settings.update", c.UpdateSettings).Ability("ai:write")
	routing.Get(r, "/operator/threads", "operator.threads.index", c.Index).Ability("conversations:read")
	routing.Post(r, "/operator/threads", "operator.threads.store", c.Store).Ability("conversations:read").
		Status(fhttp.StatusOK)
	routing.Get(r, "/operator/threads/{thread}", "operator.threads.show", c.Show).Ability("conversations:read")
	routing.Delete(r, "/operator/threads/{thread}", "operator.threads.destroy", c.Destroy).
		Ability("conversations:read").Status(fhttp.StatusNoContent)
	routing.Post(r, "/operator/threads/{thread}/messages", "operator.threads.messages.store", c.Ask).
		Ability("conversations:read").Status(fhttp.StatusOK)
	routing.Post(r, "/conversations/{conversation}/operator/brief", "conversations.operator.brief.store", c.Brief).
		Ability("conversations:read").Status(fhttp.StatusOK)
	operatorChangesetRoutes(r, a)
}

/*
operatorChangesetRoutes declares the changeset and memory routes.
*/
func operatorChangesetRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.OperatorAgent
	c := &v1.OperatorChangesetController{Changesets: acts.Changesets, Memories: acts.Memories, IDs: a.IDs}
	routing.Get(r, "/operator/changesets", "operator.changesets.index", c.Index).Ability("conversations:read")
	routing.Post(r, "/operator/changesets/{changeset}/applications", "operator.changesets.applications.store",
		c.Apply).Ability("conversations:write").Status(fhttp.StatusOK)
	routing.Post(r, "/operator/changesets/{changeset}/declines", "operator.changesets.declines.store",
		c.Decline).Ability("conversations:read").Status(fhttp.StatusOK)
	routing.Post(r, "/operator/changesets/{changeset}/reversions", "operator.changesets.reversions.store",
		c.Revert).Ability("conversations:write").Status(fhttp.StatusOK)
	routing.Get(r, "/operator/memories", "operator.memories.index", c.ListMemories).Ability("conversations:read")
	routing.Delete(r, "/operator/memories/{memory}", "operator.memories.destroy", c.Forget).
		Ability("conversations:read").Status(fhttp.StatusNoContent)
}
