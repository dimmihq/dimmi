/*
|--------------------------------------------------------------------------
| AI Routes
|--------------------------------------------------------------------------
|
| The agents' part of the API: the agents and their shared settings,
| the provider connections, the subscription connect, the runs, the
| usage and the embedding model knowledge search uses.
| Tokens need ai:read to read, ai:write to change an agent, and ai:keys
| to manage connections; the actions then check
| the member's own permissions, so changing the agent and its
| connections stays with the people who run the workspace.
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
aiRoutes declares the agent's routes.
*/
func aiRoutes(r *routing.Router, a *app.App) {
	r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

	acts := a.Actions.AI
	c := &v1.AIController{
		ListKeys:        acts.ListProviderKeys,
		CreateKey:       acts.CreateProviderKey,
		UpdateKey:       acts.UpdateProviderKey,
		RemoveKey:       acts.DeleteProviderKey,
		ValidateKey:     acts.TestProviderKey,
		StartChatGPT:    acts.StartChatGPTConnect,
		CompleteChatGPT: acts.CompleteChatGPTConnect,
		IDs:             a.IDs,
	}

	aiAgentRoutes(r, a)

	routing.Get(r, "/ai/keys", "ai.keys.index", c.Keys).Ability("ai:read")
	routing.Post(r, "/ai/keys", "ai.keys.store", c.StoreKey).Ability("ai:keys").
		Middleware("idempotent").Status(fhttp.StatusCreated)
	routing.Patch(r, "/ai/keys/{key}", "ai.keys.update", c.PatchKey).Ability("ai:keys")
	routing.Delete(r, "/ai/keys/{key}", "ai.keys.destroy", c.DeleteKey).Ability("ai:keys").
		Status(fhttp.StatusNoContent)
	routing.Post(r, "/ai/keys/{key}/test", "ai.keys.test.store", c.TestKey).Ability("ai:keys").
		Status(fhttp.StatusOK)

	routing.Post(r, "/ai/chatgpt/connect", "ai.chatgpt.connect.store", c.StartConnect).Internal().Ability("ai:keys")
	routing.Post(r, "/ai/chatgpt/callback", "ai.chatgpt.callback.store", c.CompleteConnect).
		Internal().Ability("ai:keys")

	aiTraceRoutes(r, a)
	aiActionRoutes(r, a)

	k := &v1.KnowledgeSettingsController{Settings: a.Actions.KnowledgeSettings, IDs: a.IDs}
	routing.Get(r, "/knowledge/settings", "knowledge.settings.show", k.Show).Ability("ai:read")
	routing.Patch(r, "/knowledge/settings", "knowledge.settings.update", k.Update).Ability("ai:write")
}

/*
aiTraceRoutes declares the runs and usage routes.
*/
func aiTraceRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.AI
	t := &v1.AITraceController{
		ListRuns: acts.ListRuns, ShowRun: acts.ShowRun, ShowUsage: acts.ShowUsage, IDs: a.IDs,
	}

	routing.Get(r, "/ai/usage", "ai.usage.show", t.Usage).Ability("ai:read")
	routing.Get(r, "/conversations/{conversation}/ai/runs", "conversations.ai.runs.index", t.Runs).Ability("ai:read")
	routing.Get(r, "/conversations/{conversation}/ai/runs/{run}", "conversations.ai.runs.show", t.Run).
		Ability("ai:read")
}

/*
aiAgentRoutes declares the agents, their versions, the templates a new
agent starts from and the workspace's shared AI settings.
*/
func aiAgentRoutes(r *routing.Router, a *app.App) {
	c := &v1.AIAgentController{Agents: a.Actions.AI.Agents, IDs: a.IDs}
	one := "/ai/agents/{agent}"

	routing.Get(r, "/ai/settings", "ai.settings.show", c.Settings).Ability("ai:read")
	routing.Patch(r, "/ai/settings", "ai.settings.update", c.UpdateSettings).Ability("ai:write")
	routing.Get(r, "/ai/agent-templates", "ai.agent-templates.index", c.Templates).Ability("ai:read")
	routing.Get(r, "/ai/placements", "ai.placements.index", c.Placements).Ability("ai:read")

	routing.Get(r, "/ai/agents", "ai.agents.index", c.Index).Ability("ai:read")
	routing.Post(r, "/ai/agents", "ai.agents.store", c.Store).Ability("ai:write").
		Middleware("idempotent").Status(fhttp.StatusCreated)
	routing.Get(r, one, "ai.agents.show", c.Show).Ability("ai:read")
	routing.Patch(r, one, "ai.agents.update", c.Update).Ability("ai:write")
	routing.Delete(r, one, "ai.agents.destroy", c.Destroy).Ability("ai:write").Status(fhttp.StatusNoContent)
	routing.Post(r, one+"/publications", "ai.agents.publications.store", c.Publish).
		Ability("ai:write").Status(fhttp.StatusOK)
	routing.Delete(r, one+"/publications", "ai.agents.publications.destroy", c.Unpublish).
		Ability("ai:write").Status(fhttp.StatusOK)

	aiAgentSimulationRoutes(r, a, one)

	versions := one + "/versions"
	routing.Get(r, versions, "ai.agents.versions.index", c.Versions).Ability("ai:read")
	routing.Get(r, versions+"/{version}", "ai.agents.versions.show", c.ShowVersion).Ability("ai:read")
	routing.Post(r, versions+"/{version}/restore", "ai.agents.versions.restore.store", c.RestoreVersion).
		Ability("ai:write").Status(fhttp.StatusOK)
}

/*
aiAgentSimulationRoutes declares an agent's preview and simulations.
*/
func aiAgentSimulationRoutes(r *routing.Router, a *app.App, one string) {
	if a.Actions.AI.Simulations == nil {
		return
	}
	c := &v1.AIAgentSimulationController{Simulations: a.Actions.AI.Simulations, IDs: a.IDs}
	sims := one + "/simulations"

	routing.Post(r, one+"/preview", "ai.agents.preview.store", c.Preview).Ability("ai:write").Status(fhttp.StatusOK)
	routing.Get(r, sims, "ai.agents.simulations.index", c.Index).Ability("ai:read")
	routing.Post(r, sims, "ai.agents.simulations.store", c.Store).Ability("ai:write").Status(fhttp.StatusCreated)
	routing.Post(r, sims+"/run", "ai.agents.simulations.run.store", c.Run).Ability("ai:write").Status(fhttp.StatusOK)
	routing.Post(r, sims+"/from-conversation", "ai.agents.simulations.from-conversation.store", c.Import).
		Ability("ai:write").Status(fhttp.StatusCreated)
	routing.Get(r, sims+"/{simulation}", "ai.agents.simulations.show", c.Show).Ability("ai:read")
	routing.Patch(r, sims+"/{simulation}", "ai.agents.simulations.update", c.Update).Ability("ai:write")
	routing.Delete(r, sims+"/{simulation}", "ai.agents.simulations.destroy", c.Destroy).
		Ability("ai:write").Status(fhttp.StatusNoContent)
}
