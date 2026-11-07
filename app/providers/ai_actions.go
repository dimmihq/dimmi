/*
|--------------------------------------------------------------------------
| AI Actions
|--------------------------------------------------------------------------
|
| The agent's actions on the registry: its settings, its connections,
| the subscription sign-in, the runs and usage readers, the playground
| and the workspace actions the agent may call.| playground. The playground asks whatever answers the AIPlayground
| contract, which is the pipeline once it offers a stateless entry; until
| then the playground answers that it is unavailable.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
aiActions builds the agent's actions. The answerer, the pipeline, also
serves the playground when it implements the contract.
*/
func aiActions(a *app.App, answerer any, prices *aiprices.Table) *app.AIActions {
	playground, _ := answerer.(contracts.AIPlayground)

	out := &app.AIActions{
		GetAgent:    &ai.GetAiAgent{DB: a.DB, Clock: a.Clock, Config: a.Config.AI, Prices: prices},
		UpdateAgent: &ai.UpdateAiAgent{DB: a.DB, Clock: a.Clock, Config: a.Config.AI},

		ListProviderKeys:  &ai.ListProviderKeys{DB: a.DB},
		CreateProviderKey: &ai.CreateProviderKey{DB: a.DB, Clock: a.Clock, Keys: a.Keys, LLM: a.LLM},
		UpdateProviderKey: &ai.UpdateProviderKey{DB: a.DB, Clock: a.Clock, Keys: a.Keys, LLM: a.LLM},
		DeleteProviderKey: &ai.DeleteProviderKey{DB: a.DB, Clock: a.Clock, Knowledge: a.Knowledge},
		TestProviderKey:   &ai.TestProviderKey{DB: a.DB, Clock: a.Clock, LLM: a.LLM},

		StartChatGPTConnect: &ai.StartChatGPTConnect{
			Config: a.Config.AI, Cache: a.Cache, Clock: a.Clock, HTTP: a.HTTP,
		},
		CompleteChatGPTConnect: &ai.CompleteChatGPTConnect{
			DB: a.DB, Config: a.Config.AI, Cache: a.Cache, Clock: a.Clock,
			Keys: a.Keys, HTTP: a.HTTP,
		},

		ListRuns:      &ai.ListConversationRuns{DB: a.DB},
		ShowRun:       &ai.ShowRun{DB: a.DB},
		ShowUsage:     &ai.ShowUsage{DB: a.DB, Clock: a.Clock},
		AskPlayground: &ai.AskPlayground{DB: a.DB, Playground: playground, Limiter: a.Limiter},
	}
	if runner, ok := answerer.(*pipeline.Runner); ok && runner.Actions != nil {
		addActionActions(a, out, runner.Actions)
	}
	return out
}

/*
addActionActions adds the workspace actions' own actions, and the
eraser that makes calls forget an erased customer.
*/
func addActionActions(a *app.App, out *app.AIActions, kit *actionkit.Kit) {
	out.ActionKit = kit
	out.ActionConnections = &agentactions.Connections{DB: a.DB, Clock: a.Clock, Keys: a.Keys, HTTP: a.HTTP, Kit: kit}
	out.ActionDefinitions = &agentactions.Definitions{DB: a.DB, Clock: a.Clock, Kit: kit}
	out.ActionCalls = &agentactions.Calls{DB: a.DB, Kit: kit}
	out.VisitorCalls = &widget.VisitorActionCalls{DB: a.DB, Kit: kit}
	a.ContactErasers.Register(&agentactions.CallEraser{DB: a.DB, Clock: a.Clock})
}

/*
pruneAiTraces is the nightly retention round over the agent's traces.
*/
func pruneAiTraces(a *app.App) *jobs.PruneAiTraces {
	return &jobs.PruneAiTraces{
		DB: a.DB, Clock: a.Clock, Retention: a.Retention,
		PayloadRetentionDays:    a.Config.AI.TracesRetentionDays,
		PlaygroundRetentionDays: a.Config.AI.PlaygroundRetentionDays,
		RunsRetentionDays:       a.Config.AI.RunsRetentionDays,
	}
}
