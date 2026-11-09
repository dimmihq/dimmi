package operator_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
answerModel is the model Operator answers with, priced so cost is
recorded.
*/
const answerModel = "claude-sonnet-5-5"

/*
desk is one test's workspace whose agent model is a scripted fake.
*/
type desk struct {
	t   *testing.T
	d   tests.Desk
	llm *fake.LLM
}

/*
newDesk boots a desk whose AI connection answers from a script.
*/
func newDesk(t *testing.T) *desk {
	t.Helper()
	d := tests.NewDesk(t)
	k := &desk{t: t, d: d, llm: fake.New(contracts.LLMProviderAnthropic)}
	d.TC.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return k.llm, nil
	})
	d.TC.InWorkspace(func(ctx context.Context) error {
		return connect(ctx, t, d)
	})
	return k
}

/*
connect points the workspace's agent at a fake connection.
*/
func connect(ctx context.Context, t *testing.T, d tests.Desk) error {
	t.Helper()
	now := d.TC.App.Clock.Now()
	q := models.New(d.TC.App.DB)
	ws := d.TC.Workspace.Workspace.ID
	key, err := q.CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: ws, Name: "fake", Provider: "fake", AuthType: "none", Status: "valid",
		Models: json.RawMessage("[]"), ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"),
		Source: "system", Now: now,
	})
	if err != nil {
		return err
	}
	agent, err := q.FindAgent(ctx, ws)
	if err != nil {
		agent, err = q.CreateAgent(ctx, models.CreateAgentParams{WorkspaceID: ws, Now: now})
		require.NoError(t, err)
	}
	_, err = q.UpdateAgent(ctx, models.UpdateAgentParams{
		WorkspaceID: ws, ID: agent.ID, Name: "Ada", ProductName: new("Lovelace"), Enabled: false,
		Tone: "friendly", EnabledChannels: []string{"api"}, Schedule: "always",
		AssignedBehavior: "skip_when_human_active", AnswerKeyID: &key.ID, AnswerModel: new(answerModel),
		ExcludedChannelConnectionIds: []ids.UUID{}, MaxOutputTokens: 600, MaxAiReplies: 5,
		UnknownBehavior: "offer_handoff", GroundingMode: "citations", RequireCitations: true,
		DefaultLanguage: "en", Languages: []string{}, ContactAttributeAllowlist: []string{},
		LinkAllowlist: []string{}, Streaming: true, IncludeContactContext: "basic",
		MinSimilarity: database.NumericFloat(0.35), Handoff: json.RawMessage("{}"),
		Messages: json.RawMessage("{}"), Now: now,
	})
	return err
}

/*
conversation starts a conversation from a customer and returns its
public id.
*/
func (k *desk) conversation(body string) string {
	k.t.Helper()
	res := k.d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": k.d.InboxID(), "author": "contact", "subject": "Refund", "body": body,
		"contact": testkit.JSON{"email": "ana@example.com", "name": "Ana Lima"},
	}).AssertCreated()
	return res.String("data.id")
}

/*
thread opens the owner's thread about a conversation, or about the
workspace when none is named, and returns its id.
*/
func (k *desk) thread(conversation string) string {
	k.t.Helper()
	body := testkit.JSON{}
	if conversation != "" {
		body["conversation_id"] = conversation
	}
	return k.d.Owner.PostJSON("/api/v1/operator/threads", body).AssertOK().String("data.id")
}

/*
ask asks Operator on a thread as the owner.
*/
func (k *desk) ask(thread, question string) *testkit.Response {
	k.t.Helper()
	return k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{"body": question})
}

/*
call is a scripted tool call.
*/
func call(name string, args any) contracts.ToolCall {
	raw, _ := json.Marshal(args)
	return contracts.ToolCall{ID: "call_" + name, Name: name, Arguments: raw}
}

/*
tools is a scripted model step that only calls tools.
*/
func tools(calls ...contracts.ToolCall) fake.Step {
	return fake.Step{ToolCalls: calls, Usage: contracts.LLMUsage{InputTokens: 900, OutputTokens: 30, Reported: true}}
}

/*
reply is a scripted answer.
*/
func reply(text string) fake.Step {
	return fake.Step{Text: text, Usage: contracts.LLMUsage{InputTokens: 1200, OutputTokens: 80, Reported: true}}
}

/*
count runs a counting query as the database owner.
*/
func (k *desk) count(sql string) int {
	k.t.Helper()
	var n int
	require.NoError(k.t, k.d.TC.Operator().QueryRow(k.t.Context(), sql).Scan(&n))
	return n
}

/*
fakeFailure is a scripted model call that fails.
*/
func fakeFailure() fake.Step {
	return fake.Step{Err: errors.New("provider down")}
}

/*
itoa writes an index as a JSON path segment.
*/
func itoa(i int) string {
	return strconv.Itoa(i)
}
