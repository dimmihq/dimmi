/*
|--------------------------------------------------------------------------
| Operator Routes
|--------------------------------------------------------------------------
|
| Operator, the team's own agent, over the API: its settings, each
| teammate's threads and questions, briefs on a conversation, the
| changes it proposes and what it remembers. Tokens need
| conversations:read to talk with Operator and conversations:write to
| apply or revert its changes; changing its settings needs ai:write and reading its usage ai:read.
| Routines put Operator on a clock: ai:read reads them, ai:write changes and runs them. The
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
	r.Internal()
	acts := a.Actions.OperatorAgent
	c := &v1.OperatorController{Settings: acts.Settings, Threads: acts.Threads, IDs: a.IDs}
	routing.Get(r, "/operator/settings", "operator.settings.show", c.ShowSettings).Ability("conversations:read")
	routing.Patch(r, "/operator/settings", "operator.settings.update", c.UpdateSettings).Ability("ai:write")
	routing.Get(r, "/operator/threads", "operator.threads.index", c.Index).Ability("conversations:read").Query(
		routing.QueryParam{
			Name: "filter[scope]", Values: []string{"workspace"},
			Description: "Only threads about the whole workspace.",
		},
		routing.QueryParam{
			Name: "filter[conversation]", Description: "Only threads about this conversation (conv_...).",
		},
		routing.QueryParam{
			Name: "filter[routine]", Description: "The shared threads of this routine's runs (ort_...).",
		},
	)
	routing.Post(r, "/operator/threads", "operator.threads.store", c.Store).Ability("conversations:read").
		Status(fhttp.StatusOK)
	routing.Get(r, "/operator/threads/{thread}", "operator.threads.show", c.Show).Ability("conversations:read")
	routing.Delete(r, "/operator/threads/{thread}", "operator.threads.destroy", c.Destroy).
		Ability("conversations:read").Status(fhttp.StatusNoContent)
	routing.Post(r, "/operator/threads/{thread}/messages", "operator.threads.messages.store", c.Ask).
		Ability("conversations:read").Status(fhttp.StatusOK)
	routing.Post(r, "/conversations/{conversation}/operator/brief", "conversations.operator.brief.store", c.Brief).
		Ability("conversations:read").Status(fhttp.StatusOK)
	usage := &v1.OperatorUsageController{Usage: acts.Usage}
	routing.Get(r, "/operator/usage", "operator.usage.show", usage.Show).Ability("ai:read")
	operatorChangesetRoutes(r, a)
	operatorRoutineRoutes(r, a)
}

/*
operatorChangesetRoutes declares the changeset and memory routes.
*/
func operatorChangesetRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.OperatorAgent
	c := &v1.OperatorChangesetController{Changesets: acts.Changesets, Memories: acts.Memories, IDs: a.IDs}
	routing.Get(r, "/operator/changesets", "operator.changesets.index", c.Index).Ability("conversations:read").Query(
		routing.QueryParam{
			Name: "filter[conversation]", Description: "Only changes about this conversation (conv_...).",
		},
		routing.QueryParam{
			Name: "filter[routine]", Description: "Only changes this routine suggested (ort_...).",
		},
		routing.QueryParam{
			Name: "filter[status]", Description: "Only changes in this state.",
			Values: []string{"pending", "applied", "declined", "reverted", "failed"},
		},
	)
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

/*
operatorRoutineRoutes declares the routine routes. Reading routines and
their runs needs ai:read; creating, changing, running and deleting them
needs ai:write, and the action checks ai_agent:manage.
*/
func operatorRoutineRoutes(r *routing.Router, a *app.App) {
	c := &v1.OperatorRoutineController{Routines: a.Actions.OperatorAgent.Routines, IDs: a.IDs}
	routing.Get(r, "/operator/routine-templates", "operator.routine-templates.index", c.Templates).Ability("ai:read")
	routing.Get(r, "/operator/routine-tools", "operator.routine-tools.index", c.Tools).Ability("ai:read")
	routing.Get(r, "/operator/routines", "operator.routines.index", c.Index).Ability("ai:read").Query(
		routing.QueryParam{
			Name: "filter[status]", Description: "Only routines in this state.",
			Values: []string{"active", "paused", "completed"},
		},
	)
	routing.Post(r, "/operator/routines", "operator.routines.store", c.Store).Ability("ai:write").
		Status(fhttp.StatusCreated)
	routing.Get(r, "/operator/routines/{routine}", "operator.routines.show", c.Show).Ability("ai:read")
	routing.Patch(r, "/operator/routines/{routine}", "operator.routines.update", c.Update).Ability("ai:write")
	routing.Delete(r, "/operator/routines/{routine}", "operator.routines.destroy", c.Destroy).Ability("ai:write").
		Status(fhttp.StatusNoContent)
	routing.Post(r, "/operator/routines/{routine}/runs", "operator.routines.runs.store", c.StoreRun).
		Ability("ai:write").Status(fhttp.StatusAccepted)
	routing.Get(r, "/operator/routines/{routine}/runs", "operator.routines.runs.index", c.Runs).Ability("ai:read")
	routing.Get(r, "/operator/routines/{routine}/runs/{run}", "operator.routines.runs.show", c.ShowRun).
		Ability("ai:read")
}
