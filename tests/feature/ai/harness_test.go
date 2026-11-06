package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/contracts"
	decisionfake "github.com/dimmihq/dimmi/app/drivers/decision/fake"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
The answer model the scripted agent uses, priced so cost is recorded;
its cheap sibling writes handoff notes and checks grounding.
*/
const answerModel = "claude-sonnet-5-5"

/*
seedPath is the directory of seed articles, from this package.
*/
const seedPath = "../../../database/seeders/knowledge"

/*
router is the scripted driver: answer-model calls go to one fake,
cheap-model calls to another. Hook runs before an answer stream starts;
stop overrides the answer stream's stop reason.
*/
type router struct {
	answer *fake.LLM
	cheap  *fake.LLM
	mu     sync.Mutex
	hook   func()
	stop   contracts.StopReason
}

func (r *router) Provider() contracts.LLMProvider { return contracts.LLMProviderAnthropic }

func (r *router) pick(model string) *fake.LLM {
	if model == answerModel {
		return r.answer
	}
	return r.cheap
}

func (r *router) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	return r.pick(req.Model).Chat(ctx, req)
}

func (r *router) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	r.mu.Lock()
	hook, stop := r.hook, r.stop
	r.hook = nil
	r.mu.Unlock()
	if hook != nil {
		hook()
	}
	s, err := r.pick(req.Model).Stream(ctx, req)
	if err != nil || stop == "" {
		return s, err
	}
	return &stopStream{ChatStream: s, stop: stop}, nil
}

func (r *router) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	return r.answer.ListModels(ctx)
}

func (r *router) Capabilities(model string) contracts.LLMCapabilities {
	return r.pick(model).Capabilities(model)
}

/*
stopStream reports a different stop reason on the final event.
*/
type stopStream struct {
	contracts.ChatStream
	stop contracts.StopReason
}

func (s *stopStream) Next(ctx context.Context) (contracts.StreamEvent, error) {
	ev, err := s.ChatStream.Next(ctx)
	if ev.Kind == contracts.StreamEventDone {
		ev.StopReason = s.stop
	}
	return ev, err
}

/*
harness is one test's desk with a scripted agent and decision model.
*/
type harness struct {
	t       *testing.T
	d       tests.Desk
	llm     *router
	decider *decisionfake.Decider
}

/*
newHarness boots a desk whose agent answers from the seed articles on
scripted models. Change edits the agent's settings before they are
saved.
*/
func newHarness(t *testing.T, change func(*models.UpdateAgentParams)) *harness {
	t.Helper()

	d := tests.NewDesk(t)
	decider, err := decisionfake.New(d.TC.App.Application.Config.App.Env == foundation.EnvTesting)
	require.NoError(t, err)
	h := &harness{t: t, d: d, decider: decider, llm: &router{
		answer: fake.New(contracts.LLMProviderAnthropic), cheap: fake.New(contracts.LLMProviderAnthropic),
	}}
	d.TC.App.AI.Decider = decider

	seed, err := knowledge.LoadSeed(t.Context(), d.TC.App.Storage, seedPath)
	require.NoError(t, err)
	d.TC.App.AI.Knowledge = seed

	d.TC.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return h.llm, nil
	})

	d.TC.InWorkspace(func(ctx context.Context) error {
		return saveAgent(ctx, t, d, change)
	})
	return h
}

/*
saveAgent points the workspace's agent at a fake connection.
*/
func saveAgent(ctx context.Context, t *testing.T, d tests.Desk, change func(*models.UpdateAgentParams)) error {
	t.Helper()
	now := d.TC.App.Clock.Now()
	q := models.New(d.TC.App.DB)
	key, err := q.CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: d.TC.Workspace.Workspace.ID, Name: "fake", Provider: "fake",
		AuthType: "none", Status: "valid", Models: json.RawMessage("[]"),
		ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"), Source: "system", Now: now,
	})
	if err != nil {
		return err
	}

	params := models.UpdateAgentParams{
		WorkspaceID: d.TC.Workspace.Workspace.ID, ID: agentID(ctx, t, d),
		Name: "Ada", ProductName: new("Lovelace"), Enabled: true, Tone: "friendly", Guidance: "",
		EnabledChannels: []string{"api"}, Schedule: "always", AssignedBehavior: "skip_when_human_active",
		AnswerKeyID: &key.ID, AnswerModel: new(answerModel),
		ExcludedChannelConnectionIds: []ids.UUID{}, MaxOutputTokens: 600, MaxAiReplies: 5,
		UnknownBehavior: "offer_handoff", GroundingMode: "citations", RequireCitations: true,
		DefaultLanguage: "en", Languages: []string{}, ContactAttributeAllowlist: []string{},
		LinkAllowlist: []string{}, Streaming: true, IncludeContactContext: "basic", PiiRedaction: false,
		MinSimilarity: database.NumericFloat(0.35), Handoff: json.RawMessage("{}"),
		Messages: json.RawMessage("{}"), Now: now,
	}
	if change != nil {
		change(&params)
	}
	_, err = q.UpdateAgent(ctx, params)
	return err
}

/*
agentID returns the workspace's agent id, creating the row if needed.
*/
func agentID(ctx context.Context, t *testing.T, d tests.Desk) ids.UUID {
	t.Helper()
	agent, err := models.New(d.TC.App.DB).FindAgent(ctx, d.TC.Workspace.Workspace.ID)
	if err != nil {
		agent, err = models.New(d.TC.App.DB).CreateAgent(ctx, models.CreateAgentParams{
			WorkspaceID: d.TC.Workspace.Workspace.ID, Now: d.TC.App.Clock.Now(),
		})
		require.NoError(t, err)
	}
	return agent.ID
}

/*
decide is a scripted decision: a confident intent and language, and
the named yes-or-no questions answered yes.
*/
func decide(intent, language string, yes ...string) decisionfake.Step {
	answers := map[string]contracts.DecisionAnswer{
		"intent":   {Type: contracts.DecisionChoice, Choice: intent, Confidence: 0.92},
		"language": {Type: contracts.DecisionChoice, Choice: language, Confidence: 0.95},
	}
	for _, name := range []string{"wants_human", "frustrated", "injection_suspected"} {
		answers[name] = contracts.DecisionAnswer{Type: contracts.DecisionNoul, Noul: 0.04}
	}
	for _, name := range yes {
		answers[name] = contracts.DecisionAnswer{Type: contracts.DecisionNoul, Noul: 0.93}
	}
	return decisionfake.Step{Answers: answers, Usage: contracts.DecisionUsage{InputTokens: 2_000_000}}
}

/*
summaryStep is a scripted handoff note.
*/
func summaryStep() fake.Step {
	return fake.Step{Text: "Wants: help.\nTried: nothing.\nMood: calm.\nHanded off because: asked."}
}

/*
reply is a scripted answer-model reply.
*/
func reply(text string) fake.Step {
	return fake.Step{Text: text, Usage: contracts.LLMUsage{InputTokens: 1200, OutputTokens: 80, Reported: true}}
}

/*
conversation opens a conversation from the customer and returns its id
and first message id.
*/
func (h *harness) conversation(body string) (ids.UUID, ids.UUID) {
	h.t.Helper()
	res := h.d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": h.d.InboxID(), "author": "contact", "subject": "help", "body": body,
		"contact": testkit.JSON{"email": "ana@example.com", "name": "Ana Lima"},
	}).AssertCreated()

	var convID, msgID ids.UUID
	h.d.TC.InWorkspace(func(ctx context.Context) error {
		id, err := h.d.TC.App.IDs.Decode("conv", res.String("data.id"))
		if err != nil {
			return err
		}
		convID = id
		msgs := h.messages(ctx, convID)
		if len(msgs) == 0 {
			return fmt.Errorf("first message missing")
		}
		msgID = msgs[0].ID
		return nil
	})
	return convID, msgID
}

/*
add appends a message to the conversation a second after the last one.
*/
func (h *harness) add(conv ids.UUID, author, kind, body string) ids.UUID {
	h.t.Helper()
	h.d.TC.Clock.Travel(time.Second)
	var id ids.UUID
	h.d.TC.InWorkspace(func(ctx context.Context) error {
		msg, err := models.New(h.d.TC.App.DB).CreateMessage(ctx, models.CreateMessageParams{
			WorkspaceID: h.d.TC.Workspace.Workspace.ID, ConversationID: conv, Type: kind,
			AuthorType: author, Body: &body, Substantive: kind == "message",
			Metadata: json.RawMessage("{}"), Now: h.d.TC.App.Clock.Now(),
		})
		id = msg.ID
		return err
	})
	return id
}

/*
run runs the pipeline for a trigger message.
*/
func (h *harness) run(conv, trigger ids.UUID) *pipeline.Result {
	h.t.Helper()
	h.d.TC.Clock.Travel(2 * time.Second)
	ctx := tenancy.With(h.t.Context(), h.d.TC.Workspace.Workspace.ID.String())
	res, err := h.d.TC.App.AI.Run(ctx, pipeline.Args{
		WorkspaceID: h.d.TC.Workspace.Workspace.ID, ConversationID: conv,
		TriggerMessageID: trigger, Kind: "conversation", QueuedAt: h.d.TC.App.Clock.Now(),
	})
	require.NoError(h.t, err)
	return res
}

/*
operator runs raw SQL as the operator.
*/
func (h *harness) operator(sql string, args ...any) {
	h.t.Helper()
	_, err := h.d.TC.Operator().Exec(h.t.Context(), sql, args...)
	require.NoError(h.t, err)
}

/*
messages lists the conversation's timeline, notes included.
*/
func (h *harness) messages(ctx context.Context, conv ids.UUID) []models.Message {
	h.t.Helper()
	msgs, err := models.New(h.d.TC.App.DB).ListMessages(ctx, models.ListMessagesParams{
		WorkspaceID: h.d.TC.Workspace.Workspace.ID, ConversationID: conv, RowLimit: 100,
	})
	require.NoError(h.t, err)
	return msgs
}

/*
state is everything a test asserts on after a run.
*/
type state struct {
	conv     models.Conversation
	messages []models.Message
	run      models.AiRun
	steps    []models.AiRunStep
	runs     []models.AiRun
}

/*
agentReplies are the agent's public messages.
*/
func (s state) agentReplies() []models.Message {
	var out []models.Message
	for _, m := range s.messages {
		if m.AuthorType == "agent" && m.Type == "message" {
			out = append(out, m)
		}
	}
	return out
}

/*
notes are the conversation's internal notes.
*/
func (s state) notes() []models.Message {
	var out []models.Message
	for _, m := range s.messages {
		if m.Type == "note" {
			out = append(out, m)
		}
	}
	return out
}

/*
stepKinds lists the run's step kinds in order.
*/
func (s state) stepKinds() []string {
	out := make([]string, len(s.steps))
	for i, st := range s.steps {
		out[i] = st.Kind
	}
	return out
}

/*
load reads the conversation, its messages, its runs and the given
run's steps.
*/
func (h *harness) load(conv, runID ids.UUID) state {
	h.t.Helper()
	var s state
	ws := h.d.TC.Workspace.Workspace.ID
	h.d.TC.InWorkspace(func(ctx context.Context) error {
		q := models.New(h.d.TC.App.DB)
		var err error
		if s.conv, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conv}); err != nil {
			return err
		}
		s.messages = h.messages(ctx, conv)
		if s.runs, err = q.ListConversationRuns(ctx, models.ListConversationRunsParams{
			WorkspaceID: ws, ConversationID: &conv, PageLimit: 50,
		}); err != nil {
			return err
		}
		if runID == (ids.UUID{}) {
			return nil
		}
		if s.run, err = q.FindRun(ctx, models.FindRunParams{WorkspaceID: ws, ID: runID}); err != nil {
			return err
		}
		s.steps, err = q.ListRunSteps(ctx, models.ListRunStepsParams{WorkspaceID: ws, RunID: runID})
		return err
	})
	return s
}

/*
sentText joins every text a fake received: system blocks and turns.
*/
func sentText(llms ...*fake.LLM) string {
	var b strings.Builder
	for _, l := range llms {
		for _, req := range l.Sent() {
			for _, s := range req.System {
				b.WriteString(s.Text + "\n")
			}
			for _, m := range req.Messages {
				for _, p := range m.Parts {
					b.WriteString(p.Text + "\n")
				}
			}
		}
	}
	return b.String()
}

/*
aiMeta reads a message's agent metadata.
*/
func aiMeta(t *testing.T, m models.Message) map[string]any {
	t.Helper()
	var meta struct {
		AI map[string]any `json:"ai"`
	}
	require.NoError(t, json.Unmarshal(m.Metadata, &meta))
	return meta.AI
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
