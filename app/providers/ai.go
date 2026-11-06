/*
|--------------------------------------------------------------------------
| AI Agent
|--------------------------------------------------------------------------
|
| The agent's wiring: the knowledge contract starts on its stand-in
| while the index is built, the manager resolves connections to
| drivers, and every AI action lands on the registry.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/managers"
)

/*
bootAI wires the agent's services and actions.
*/
func bootAI(a *app.App) {
	if a.Knowledge == nil {
		a.Knowledge = knowledge.NewStub()
	}
	if a.LLM == nil {
		a.LLM = managers.NewLLMManager(managers.LLMDeps{
			Keys: a.Keys, HTTP: a.HTTP, Config: a.Config.AI, Clock: a.Clock,
		})
	}

	a.Actions.AI = &app.AIActions{
		GetAgent:    &ai.GetAiAgent{DB: a.DB, Clock: a.Clock, Config: a.Config.AI},
		UpdateAgent: &ai.UpdateAiAgent{DB: a.DB, Clock: a.Clock, Config: a.Config.AI},

		ListProviderKeys:  &ai.ListProviderKeys{DB: a.DB},
		CreateProviderKey: &ai.CreateProviderKey{DB: a.DB, Clock: a.Clock, Keys: a.Keys, LLM: a.LLM},
		UpdateProviderKey: &ai.UpdateProviderKey{DB: a.DB, Clock: a.Clock, Keys: a.Keys, LLM: a.LLM},
		DeleteProviderKey: &ai.DeleteProviderKey{DB: a.DB, Clock: a.Clock, Knowledge: a.Knowledge},
		TestProviderKey:   &ai.TestProviderKey{DB: a.DB, Clock: a.Clock, LLM: a.LLM},

		StartChatGPTConnect: &ai.StartChatGPTConnect{
			Config: a.Config.AI, Cache: a.Cache, Clock: a.Clock,
		},
		CompleteChatGPTConnect: &ai.CompleteChatGPTConnect{
			DB: a.DB, Config: a.Config.AI, Cache: a.Cache, Clock: a.Clock,
			Keys: a.Keys, HTTP: a.HTTP,
		},
	}
}
