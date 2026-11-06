/*
|--------------------------------------------------------------------------
| AI Agent
|--------------------------------------------------------------------------
|
| The agent's wiring: the knowledge contract starts on its stand-in
| while the index is built, the manager resolves connections to
| drivers, the runner carries a customer's message through the
| pipeline, and every AI action lands on the registry.
|
*/

package providers

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/knowledge"
	aijobs "github.com/dimmihq/dimmi/app/ai/jobs"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/app/support/prompts"
	airesources "github.com/dimmihq/dimmi/resources/ai"
)

/*
bootAI wires the agent's services, actions, listener and job.
*/
func bootAI(a *app.App) error {
	if a.Knowledge == nil {
		a.Knowledge = knowledge.NewStub()
	}
	if a.LLM == nil {
		a.LLM = managers.NewLLMManager(managers.LLMDeps{
			Keys: a.Keys, HTTP: a.HTTP, Config: a.Config.AI, Clock: a.Clock, DB: a.DB,
		})
	}

	runner := &pipeline.Runner{
		DB: a.DB, Clock: a.Clock, LLM: a.LLM, Knowledge: a.Knowledge,
		Prompts: prompts.NewLoader(a.Config.AI.Prompts.Version, nil),
		Prices:  loadPrices(a),
		Events:  a.Events,
	}
	a.AI = runner

	a.Actions.AI = &app.AIActions{
		GetAgent:    &ai.GetAiAgent{DB: a.DB, Clock: a.Clock, Config: a.Config.AI, Prices: runner.Prices},
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
	}

	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.MessageCreated) error {
		return enqueueAnswer(ctx, a, runner, e)
	})

	return errors.Join(
		queue.Register(a.Queue, &aijobs.AnswerWithAI{Runner: runner}),
		queue.Register(a.Queue, &jobs.PruneAiTraces{
			DB: a.DB, Clock: a.Clock,
			StepsRetentionDays: a.Config.AI.TracesRetentionDays,
			RunsRetentionDays:  a.Config.AI.RunsRetentionDays,
		}),
	)
}

/*
enqueueAnswer dispatches the answer job for a substantive customer
message, a debounce after it landed so rapid typing gathers.
*/
func enqueueAnswer(ctx context.Context, a *app.App, _ *pipeline.Runner,
	e appevents.MessageCreated,
) error {
	if e.Type != "message" || e.AuthorType != "contact" || !e.Substantive {
		return nil
	}
	if !a.Config.AI.Enabled {
		return nil
	}

	debounce := debounceFor(a, e.ChannelType)

	return a.Queue.Push(context.WithoutCancel(ctx), aijobs.AnswerWithAIArgs{
		WorkspaceID:      e.WorkspaceID,
		ConversationID:   e.ConversationID,
		TriggerMessageID: e.MessageID,
		RunKind:          "conversation",
		QueuedAt:         a.Clock.Now().Unix(),
	}, queue.At(a.Clock.Now().Add(time.Duration(debounce)*time.Millisecond)),
		queue.Unique(queue.UniqueOpts{ByArgs: true}))
}

/*
debounceFor picks the channel's gather window.
*/
func debounceFor(a *app.App, channelType string) int {
	switch channelType {
	case "widget":
		return a.Config.AI.DebounceWidgetMS
	case "email":
		return a.Config.AI.DebounceEmailMS
	default:
		return a.Config.AI.DebounceChatMS
	}
}

/*
loadPrices reads the shipped price table.
*/
func loadPrices(_ *app.App) *aiprices.Table {
	data, err := airesources.Files.ReadFile("prices.json")
	if err != nil {
		return nil
	}
	table, err := aiprices.Parse(data)
	if err != nil {
		return nil
	}
	return table
}
