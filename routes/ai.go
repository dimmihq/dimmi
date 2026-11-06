/*
|--------------------------------------------------------------------------
| AI Routes
|--------------------------------------------------------------------------
|
| The agent's part of the API: its settings, its provider connections
| and the subscription connect. Reading needs ai:read, changing the
| agent ai:write, and managing connections ai:keys:write.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
aiRoutes declares the agent's routes.
*/
func aiRoutes(r *routing.Router, a *app.App) {
	r.Middleware("csrf", "session", "auth", "workspace", "api.throttle")

	c := &v1.AIController{
		Get:             a.Actions.AI.GetAgent,
		Update:          a.Actions.AI.UpdateAgent,
		ListKeys:        a.Actions.AI.ListProviderKeys,
		CreateKey:       a.Actions.AI.CreateProviderKey,
		UpdateKey:       a.Actions.AI.UpdateProviderKey,
		RemoveKey:       a.Actions.AI.DeleteProviderKey,
		ValidateKey:     a.Actions.AI.TestProviderKey,
		StartChatGPT:    a.Actions.AI.StartChatGPTConnect,
		CompleteChatGPT: a.Actions.AI.CompleteChatGPTConnect,
		IDs:             a.IDs,
		URL:             a.Application.Config.App.URL,
	}

	routing.Get(r, "/ai/agent", "api.v1.ai.agent.show", c.Agent).Ability("ai:read")
	routing.Patch(r, "/ai/agent", "api.v1.ai.agent.update", c.UpdateAgent).Ability("ai:write")

	routing.Get(r, "/ai/keys", "api.v1.ai.keys.index", c.Keys).Ability("ai:read")
	routing.Post(r, "/ai/keys", "api.v1.ai.keys.store", c.StoreKey).Ability("ai:keys")
	routing.Patch(r, "/ai/keys/{key}", "api.v1.ai.keys.update", c.PatchKey).Ability("ai:keys")
	routing.Delete(r, "/ai/keys/{key}", "api.v1.ai.keys.destroy", c.DeleteKey).Ability("ai:keys")
	routing.Post(r, "/ai/keys/{key}/test", "api.v1.ai.keys.test.store", c.TestKey).Ability("ai:keys")

	routing.Post(r, "/ai/chatgpt/connect", "api.v1.ai.chatgpt.connect.store", c.StartConnect).Ability("ai:keys")
	routing.Post(r, "/ai/chatgpt/callback", "api.v1.ai.chatgpt.callback.store", c.CompleteConnect).Ability("ai:keys")

	routing.Get(r, "/ai/usage", "api.v1.ai.usage.show", c.Usage).Ability("ai:read")
	routing.Get(r, "/conversations/{conversation}/ai/runs", "api.v1.ai.runs.index", c.Runs).Ability("ai:read")
	routing.Get(r, "/conversations/{conversation}/ai/runs/{run}", "api.v1.ai.runs.show", c.Run).Ability("ai:read")
}
