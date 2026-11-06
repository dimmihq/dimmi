/*
|--------------------------------------------------------------------------
| AI Routes
|--------------------------------------------------------------------------
|
| The agent's part of the API: its settings, its provider connections,
| the subscription connect, its runs, its usage and the playground.
| Tokens need ai:read to read, ai:write to change the agent or ask the
| playground, and ai:keys to manage connections; the actions then check
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
		Get:             acts.GetAgent,
		Update:          acts.UpdateAgent,
		ListKeys:        acts.ListProviderKeys,
		CreateKey:       acts.CreateProviderKey,
		UpdateKey:       acts.UpdateProviderKey,
		RemoveKey:       acts.DeleteProviderKey,
		ValidateKey:     acts.TestProviderKey,
		StartChatGPT:    acts.StartChatGPTConnect,
		CompleteChatGPT: acts.CompleteChatGPTConnect,
		IDs:             a.IDs,
		URL:             a.Application.Config.App.URL,
	}

	routing.Get(r, "/ai/agent", "api.v1.ai.agent.show", c.Agent).Ability("ai:read")
	routing.Patch(r, "/ai/agent", "api.v1.ai.agent.update", c.UpdateAgent).Ability("ai:write")

	routing.Get(r, "/ai/keys", "api.v1.ai.keys.index", c.Keys).Ability("ai:read")
	routing.Post(r, "/ai/keys", "api.v1.ai.keys.store", c.StoreKey).Ability("ai:keys").
		Middleware("idempotent").Status(fhttp.StatusCreated)
	routing.Patch(r, "/ai/keys/{key}", "api.v1.ai.keys.update", c.PatchKey).Ability("ai:keys")
	routing.Delete(r, "/ai/keys/{key}", "api.v1.ai.keys.destroy", c.DeleteKey).Ability("ai:keys").
		Status(fhttp.StatusNoContent)
	routing.Post(r, "/ai/keys/{key}/test", "api.v1.ai.keys.test.store", c.TestKey).Ability("ai:keys").
		Status(fhttp.StatusOK)

	routing.Post(r, "/ai/chatgpt/connect", "api.v1.ai.chatgpt.connect.store", c.StartConnect).Ability("ai:keys")
	routing.Post(r, "/ai/chatgpt/callback", "api.v1.ai.chatgpt.callback.store", c.CompleteConnect).Ability("ai:keys")

	aiTraceRoutes(r, a)
}

/*
aiTraceRoutes declares the runs, usage and playground routes.
*/
func aiTraceRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.AI
	t := &v1.AITraceController{
		ListRuns: acts.ListRuns, ShowRun: acts.ShowRun, ShowUsage: acts.ShowUsage, Playground: acts.AskPlayground,
		IDs: a.IDs,
	}

	routing.Get(r, "/ai/usage", "api.v1.ai.usage.show", t.Usage).Ability("ai:read")
	routing.Post(r, "/ai/playground", "api.v1.ai.playground.store", t.Ask).Ability("ai:write").Status(fhttp.StatusOK)
	routing.Get(r, "/conversations/{conversation}/ai/runs", "api.v1.ai.runs.index", t.Runs).Ability("ai:read")
	routing.Get(r, "/conversations/{conversation}/ai/runs/{run}", "api.v1.ai.runs.show", t.Run).Ability("ai:read")
}
