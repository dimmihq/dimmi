package channels_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
answerModel is the model the scripted agent answers and classifies
with, priced so cost is recorded.
*/
const answerModel = "claude-sonnet-5-5"

/*
withAgent gives the workspace an agent on a scripted fake model that
answers from the seed articles, and returns the fake.
*/
func withAgent(sc *slackCase) *fake.LLM {
	sc.T.Helper()

	llm := fake.New(contracts.LLMProviderAnthropic)

	seed, err := knowledge.LoadSeed(sc.T.Context(), sc.App.Storage, "../../../database/seeders/knowledge")
	require.NoError(sc.T, err)
	sc.App.AI.Knowledge = seed

	sc.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return llm, nil
	})

	sc.InWorkspace(func(ctx context.Context) error { return saveAgent(ctx, sc) })

	return llm
}

/*
saveAgent points the workspace's agent at a fake connection, answering
on Slack.
*/
func saveAgent(ctx context.Context, sc *slackCase) error {
	now := sc.Clock.Now()
	q := models.New(sc.App.DB)

	key, err := q.CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: sc.Workspace.Workspace.ID, Name: "fake", Provider: "fake", AuthType: "none", Status: "valid",
		Models: json.RawMessage("[]"), ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"),
		Source: "system", Now: now,
	})
	if err != nil {
		return err
	}

	agent, err := q.FindAgent(ctx, sc.Workspace.Workspace.ID)
	if err != nil {
		agent, err = q.CreateAgent(ctx, models.CreateAgentParams{WorkspaceID: sc.Workspace.Workspace.ID, Now: now})
	}

	if err != nil {
		return err
	}

	_, err = q.UpdateAgent(ctx, models.UpdateAgentParams{
		WorkspaceID: sc.Workspace.Workspace.ID, ID: agent.ID, Name: "Ada", ProductName: new("Lovelace"),
		Enabled: true, Tone: "friendly", EnabledChannels: []string{"slack"}, Schedule: "always",
		AssignedBehavior: "skip_when_human_active", AnswerKeyID: &key.ID, AnswerModel: new(answerModel),
		ExcludedChannelConnectionIds: []ids.UUID{}, MaxOutputTokens: 600, MaxAiReplies: 5,
		UnknownBehavior: "offer_handoff", GroundingMode: "citations", RequireCitations: true, DefaultLanguage: "en",
		Languages: []string{}, ContactAttributeAllowlist: []string{}, LinkAllowlist: []string{}, Streaming: true,
		IncludeContactContext: "basic", MinSimilarity: database.NumericFloat(0.35), Handoff: json.RawMessage("{}"),
		Messages: json.RawMessage("{}"), Now: now,
	})

	return err
}

/*
runAgent runs the agent on the conversation's first message.
*/
func (sc *slackCase) runAgent(conv models.Conversation, delayed bool) *pipeline.Result {
	sc.T.Helper()

	sc.Clock.Travel(3 * time.Second)

	ctx := tenancy.With(sc.T.Context(), sc.Workspace.Workspace.ID.String())
	res, err := sc.App.AI.Run(ctx, pipeline.Args{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, TriggerMessageID: sc.messages(conv)[0].ID,
		Kind: "conversation", QueuedAt: sc.Clock.Now(), Delayed: delayed,
	})
	require.NoError(sc.T, err)

	return res
}

/*
run returns the run with the id.
*/
func (sc *slackCase) run(id ids.UUID) models.AiRun {
	return query(sc, func(ctx context.Context, q *models.Queries) (models.AiRun, error) {
		return q.FindRun(ctx, models.FindRunParams{WorkspaceID: sc.Workspace.Workspace.ID, ID: id})
	})
}

/*
answer is a scripted answer over the seed articles.
*/
func answer(llm *fake.LLM) {
	query, _ := json.Marshal(map[string]string{"query": "pro plan price per seat"})
	llm.Script(
		fake.Step{
			ToolCalls: []contracts.ToolCall{{ID: "call_search", Name: "search_knowledge", Arguments: query}},
			Usage:     contracts.LLMUsage{InputTokens: 900, OutputTokens: 30, Reported: true},
		},
		fake.Step{
			Text:  "The Pro plan costs $12 per seat per month [1].",
			Usage: contracts.LLMUsage{InputTokens: 1200, OutputTokens: 80, Reported: true},
		},
	)
}

func TestSlackAgentControls(t *testing.T) {
	t.Parallel()

	t.Run("in an auto room the agent's answer is posted in the customer's thread", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)
		answer(withAgent(sc))

		res := sc.runAgent(conv, false)
		require.Equal(t, "answered", res.Outcome)
		sc.Drain()

		replies := sc.posted(supportRoom)
		require.NotEmpty(t, replies)
		answered := replies[len(replies)-1]
		assert.Equal(t, rootTS, answered.Form.Get("thread_ts"))
		assert.Contains(t, answered.Form.Get("blocks"), `"value":"solved"`)
		assert.Contains(t, answered.Form.Get("blocks"), `"value":"handoff"`)
		assert.Contains(t, answered.Form.Get("blocks"), "plans-and-pricing|Plans and pricing")

		agentAnswer := sc.agentAnswer(conv)
		require.NotNil(t, agentAnswer.DeliveryStatus)
		assert.Equal(t, string(enums.DeliveryStatusSent), *agentAnswer.DeliveryStatus)

		sc.interact(pressed(agentAnswer, contracts.ControlSolved, "U0CUSTOMER"))
		cleared := sc.lastUpdate(supportRoom)
		require.NotEmpty(t, cleared)
		assert.Contains(t, cleared, "plans-and-pricing|Plans and pricing", "the sources stay once the buttons go")
		assert.NotContains(t, cleared, `"value":"solved"`)
	})

	t.Run("an answer the agent could not find offers a human and waits for the customer", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)
		query, _ := json.Marshal(map[string]string{"query": "weather paris"})
		withAgent(sc).Script(
			fake.Step{ToolCalls: []contracts.ToolCall{{ID: "c1", Name: "search_knowledge", Arguments: query}}},
			fake.Step{Text: "I couldn't find anything about that in our docs."},
		)

		res := sc.runAgent(conv, false)
		require.Equal(t, "unknown", res.Outcome)
		sc.Drain()

		replies := sc.posted(supportRoom)
		require.NotEmpty(t, replies)
		blocks := replies[len(replies)-1].Form.Get("blocks")
		assert.Contains(t, blocks, `"value":"handoff"`)
		assert.NotContains(t, blocks, `"value":"solved"`)
		assert.Equal(t, string(enums.AIStateActive), sc.refreshed(conv).AiState)
	})

	t.Run("in a draft room the answer is a draft the teammate sends as themselves", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)
		answer(withAgent(sc))
		sc.setSetting("agent_mode", "draft")

		res := sc.runAgent(conv, false)
		require.Equal(t, "drafted", res.Outcome)
		run := sc.run(res.RunID)
		require.NotNil(t, run.DraftBody)
		assert.Equal(t, int32(0), sc.refreshed(conv).AiReplyCount)

		sc.Drain()
		drafts := sc.posted(triageRoom)
		require.Len(t, drafts, 2)
		assert.Equal(t, postedTS, drafts[1].Form.Get("thread_ts"))

		send := slackdriver.TeamActionID(slackdriver.DraftPrefix(), models.PublicID(models.PrefixAiRun, run.ID),
			slackdriver.DraftSend)
		sc.interact(teamPress(send, "U0TEAMMATE", ""))
		sc.interact(teamPress(send, "U0TEAMMATE", ""))

		var sent []models.Message
		for _, m := range sc.messages(conv) {
			if m.AuthorType == string(enums.AuthorTypeMember) && m.Type == string(enums.MessageTypeMessage) {
				sent = append(sent, m)
			}
		}

		require.Len(t, sent, 1, "a draft is sent once")
		assert.Equal(t, *run.DraftBody, body(sent[0]))
		require.NotNil(t, sc.run(run.ID).ShadowAgreed)
		assert.True(t, *sc.run(run.ID).ShadowAgreed)

		agreed, judged, err := sc.App.Actions.Slack.Channel.ShadowScore(t.Context(), conv.WorkspaceID,
			sc.room(supportRoom).ID)
		require.NoError(t, err)
		assert.Equal(t, [2]int64{1, 1}, [2]int64{agreed, judged})
	})

	t.Run("discarding a draft records it on the run", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)
		answer(withAgent(sc))
		sc.setSetting("agent_mode", "draft")
		res := sc.runAgent(conv, false)
		sc.Drain()

		sc.interact(teamPress(slackdriver.TeamActionID(slackdriver.DraftPrefix(),
			models.PublicID(models.PrefixAiRun, res.RunID), slackdriver.DraftDiscard), "U0TEAMMATE", ""))

		run := sc.run(res.RunID)
		require.NotNil(t, run.Feedback)
		assert.Equal(t, "discarded", *run.Feedback)
		assert.NotNil(t, run.FeedbackAt)
	})

	t.Run("a teammate answering during the room's delay skips the agent with that reason", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		withAgent(sc)
		sc.setSetting("agent_delay_seconds", 120)
		sc.setSetting("agent_delay_when", "always")

		delay, insideHours := sc.App.Actions.Slack.Channel.AgentDelay(t.Context(), conv.WorkspaceID, conv.ID)
		assert.Equal(t, 2*time.Minute, delay)
		assert.False(t, insideHours)

		sc.deliver(reply(message(supportRoom, "U0TEAMMATE", secondTS, "On it"), rootTS))
		res := sc.runAgent(conv, true)
		assert.Equal(t, "human_replied_in_delay", res.SkipReason)
	})

	t.Run("the working state uses Slack's agent session, the eyes in shared rooms", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.deliver(message(communityRoom, "U0CUSTOMER", rootTS, "The export fails"))
		conv := sc.conversations()[0]
		first := sc.messages(conv)[0]

		sc.App.Actions.Slack.Channel.Started(t.Context(), conv.WorkspaceID, conv.ID, first.ID)
		sc.App.Actions.Slack.Channel.Stopped(t.Context(), conv.WorkspaceID, conv.ID, first.ID)

		sessions := sc.slack.called("agents.sessions.setStatus")
		require.Len(t, sessions, 2)
		assert.Equal(t, "processing", sessions[0].Form.Get("status"))
		assert.Equal(t, "active", sessions[1].Form.Get("status"))
		assert.Empty(t, sc.slack.called("reactions.add"))
	})
}

func TestSlackRequestModel(t *testing.T) {
	t.Parallel()

	t.Run("the workspace's model sorts chatter out and records a classify run", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		withAgent(sc).Script(fake.Step{
			Text: `{"kind": "chatter", "confidence": 0.95}`, Usage: contracts.LLMUsage{InputTokens: 80, Reported: true},
		})
		sc.setRoom(supportRoom, "shared", "ai")

		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "Haha, see you all at the offsite"))

		assert.Empty(t, sc.conversations())

		var kinds int
		require.NoError(t, sc.Operator().QueryRow(t.Context(),
			"SELECT count(*) FROM ai_runs WHERE workspace_id = $1 AND kind = 'classify' AND status = 'completed'",
			sc.Workspace.Workspace.ID).Scan(&kinds))
		assert.Equal(t, 1, kinds)
	})

	t.Run("a model that cannot keep to a schema is never asked", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		llm := withAgent(sc)
		llm.CapabilitiesOf(contracts.LLMCapabilities{Tools: true, Streaming: true})
		sc.setRoom(supportRoom, "shared", "ai")

		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))

		assert.Len(t, sc.conversations(), 1)
		assert.Empty(t, llm.Sent())
	})

	t.Run("without a model the room groups by author instead", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "ai")

		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))

		assert.Len(t, sc.conversations(), 1)
	})
}

/*
agentAnswer returns the agent's newest answer in the conversation.
*/
func (sc *slackCase) agentAnswer(conv models.Conversation) models.Message {
	sc.T.Helper()

	var out models.Message
	for _, m := range sc.messages(conv) {
		if m.AuthorType == string(enums.AuthorTypeAgent) && m.Type == string(enums.MessageTypeMessage) {
			out = m
		}
	}

	return out
}

/*
lastUpdate returns the blocks of the newest message update in the
channel, empty for none.
*/
func (sc *slackCase) lastUpdate(channel string) string {
	var out string
	for _, u := range sc.slack.called("chat.update") {
		if u.Form.Get("channel") == channel {
			out = u.Form.Get("blocks")
		}
	}

	return out
}
