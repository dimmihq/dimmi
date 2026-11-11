/*
|--------------------------------------------------------------------------
| AI Agent
|--------------------------------------------------------------------------
|
| The agent's wiring: the knowledge contract starts on the seeded
| index while the help center's index is built, the manager resolves
| connections to drivers, the prompt tree is checked before anything
| runs, the runner carries a customer's message through the pipeline
| with the workspace's own actions as tools, and every AI action lands
| on the registry.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	aijobs "github.com/dimmihq/dimmi/app/ai/jobs"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/ai/verdicts"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/app/support/prompts"
	airesources "github.com/dimmihq/dimmi/resources/ai"
)

/*
bootAI wires the agent's services, actions, listener and job.
*/
func bootAI(ctx context.Context, a *app.App) error {
	runner, err := newRunner(ctx, a)
	if err != nil {
		return err
	}
	a.AI = runner
	runner.Working = a.Actions.Slack.Channel
	runner.Delivery = &conversations.Delivery{Channels: a.Channels, Queue: a.Queue}
	runner.Drafts = a.Actions.Slack.Channel
	cheap := &verdicts.Verdicts{DB: a.DB, Clock: a.Clock, LLM: a.LLM, Prices: runner.Prices}
	a.Actions.Slack.Channel.Classifier = &verdicts.Requests{Verdicts: cheap}
	a.Actions.Slack.Channel.Judge = &verdicts.Judge{Verdicts: cheap}
	if a.Actions.Series != nil {
		a.Actions.Series.Picker = &verdicts.Paths{Verdicts: cheap}
	}
	runner.Actions = &actionkit.Kit{
		DB: a.DB, Clock: a.Clock, HTTP: a.HTTP, Keys: a.Keys, Events: a.Events, Queue: a.Queue,
	}

	a.Actions.AI = aiActions(a, runner, runner.Prices)

	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.MessageCreated) error {
		return enqueueAnswer(ctx, a, e)
	})
	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.WidgetEmailProven) error {
		return runner.Actions.EmailProven(tenancy.With(ctx, e.WorkspaceID.String()), e.WorkspaceID, e.ContactID,
			e.Email)
	})

	return errors.Join(
		queue.Register(a.Queue, &aijobs.AnswerWithAI{Runner: runner}),
		queue.Register(a.Queue, &aijobs.AnswerForWorkflow{Runner: runner}),
		queue.Register(a.Queue, pruneAiTraces(a)),
		queue.Register(a.Queue, &aijobs.ExecuteAiActionCall{Kit: runner.Actions}),
		queue.Register(a.Queue, &aijobs.FollowUpAiActionCall{Runner: runner}),
		queue.Register(a.Queue, &aijobs.ExpireAiActionCall{Kit: runner.Actions}),
	)
}

/*
newRunner builds the pipeline runner: the knowledge index over the
workspace's published articles, falling back to the seed, the driver
manager and the checked prompt tree. A broken prompt tree or an
unreadable seed directory stops the boot.
*/
func newRunner(ctx context.Context, a *app.App) (*pipeline.Runner, error) {
	if a.Knowledge == nil {
		seed, err := knowledge.LoadSeed(ctx, a.Storage, a.Config.AI.KnowledgeSeedPath)
		if err != nil {
			return nil, err
		}
		a.Knowledge = &helpcenters.Knowledge{
			DB: a.DB, Fallback: seed, Vectors: a.Vectors, Addresses: helpcenters.Addresses{
				BaseDomain: a.Config.HelpCenters.BaseDomain, AppURL: a.Application.Config.App.URL,
			},
		}
	}
	if a.LLM == nil {
		a.LLM = managers.NewLLMManager(managers.LLMDeps{
			Keys: a.Keys, HTTP: a.HTTP, Config: a.Config.AI, Clock: a.Clock, DB: a.DB,
		})
	}

	loader, err := prompts.NewLoader(a.Config.AI.Prompts.Version, nil)
	if err != nil {
		return nil, fmt.Errorf("load agent prompts: %w", err)
	}

	return &pipeline.Runner{
		DB: a.DB, Clock: a.Clock, LLM: a.LLM, Knowledge: a.Knowledge, Prompts: loader,
		Prices: loadPrices(a), Events: a.Events, Publisher: a.Broadcast,
		Timeouts: a.Config.AI.Timeouts, StorePayloads: a.Config.AI.TracesStorePayloads,
	}, nil
}

/*
enqueueAnswer queues the answer job for a customer message, a
debounce after it landed. Every message gets its own job, unique to
that message: in a burst, each job but the newest steps aside and the
newest answers them all. Conversations the agent can never answer
queue nothing.
*/
func enqueueAnswer(ctx context.Context, a *app.App, e appevents.MessageCreated) error {
	if e.Type != "message" || e.AuthorType != "contact" || !e.Substantive || !a.Config.AI.Enabled {
		return nil
	}

	target, err := answerable(ctx, a, e)
	if err != nil || !target.eligible {
		return err
	}

	debounce := time.Duration(debounceFor(a, target.channel)) * time.Millisecond
	delay := agentDelay(ctx, a, e, target)
	return a.Queue.Push(context.WithoutCancel(ctx), aijobs.AnswerWithAIArgs{
		WorkspaceID:      e.WorkspaceID,
		ConversationID:   e.ConversationID,
		TriggerMessageID: e.MessageID,
		RunKind:          "conversation",
		QueuedAt:         a.Clock.Now().UnixMilli(),
		Delayed:          delay > debounce,
	}, queue.At(a.Clock.Now().Add(max(debounce, delay))), queue.Unique(queue.UniqueOpts{ByArgs: true}))
}

/*
agentDelay is how long the conversation's place holds the agent back so
a teammate can answer first, zero for none. A place that only holds it
back inside office hours lets it answer at once outside them.
*/
func agentDelay(ctx context.Context, a *app.App, e appevents.MessageCreated, t answerTarget) time.Duration {
	if t.channel != "slack" {
		return 0
	}
	delay, insideHoursOnly := a.Actions.Slack.Channel.AgentDelay(ctx, e.WorkspaceID, e.ConversationID)
	if delay <= 0 || !insideHoursOnly {
		return delay
	}
	if !t.open {
		return 0
	}
	return delay
}

/*
answerTarget is what the cheap check found: the conversation's channel
type, whether the agent may answer it, the agent, and for Slack whether
the conversation's inbox is inside its office hours now.
*/
type answerTarget struct {
	channel  string
	eligible bool
	open     bool
	agent    models.AiAgent
}

/*
answerable is the cheap check before queueing: the workspace has an
enabled agent and the conversation is live and not handed to the
team. It returns the conversation's channel type for the debounce.
*/
func answerable(ctx context.Context, a *app.App, e appevents.MessageCreated) (answerTarget, error) {
	var t answerTarget
	ctx = tenancy.With(ctx, e.WorkspaceID.String())

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		agent, err := q.FindAgent(ctx, e.WorkspaceID)
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		conv, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: e.WorkspaceID, ID: e.ConversationID,
		})
		if err != nil {
			return err
		}
		t.agent = agent
		t.eligible = agent.Enabled && conv.AiState != "handed_off" && conv.DeletedAt == nil && !conv.Spam
		if conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: e.WorkspaceID, ID: conv.ChannelConnectionID,
		}); err == nil {
			t.channel = conn.Type
		}
		if t.channel == "slack" {
			place := schedules.Place{Workspace: e.WorkspaceID, Inbox: conv.InboxID}
			t.open, _ = schedules.OpenAt(ctx, q, place, a.Clock)
		}
		return nil
	})
	if err != nil {
		return t, fmt.Errorf("check answerable: %w", err)
	}
	return t, nil
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
