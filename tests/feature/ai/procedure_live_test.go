package ai_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/anthropic"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
liveModel is the real model the live procedure tests answer with.
*/
const liveModel = "claude-sonnet-5"

/*
liveLLM is the real Anthropic driver, recording the tools each answer
called so a test can read what the model did.
*/
type liveLLM struct {
	*anthropic.Driver
	mu    sync.Mutex
	calls []string
}

func (l *liveLLM) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	s, err := l.Driver.Stream(ctx, req)
	if err != nil {
		return s, err
	}
	return &recordedStream{ChatStream: s, llm: l}, nil
}

/*
called lists the tools the model called, in order.
*/
func (l *liveLLM) called() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.calls...)
}

/*
recordedStream notes each tool call the model makes.
*/
type recordedStream struct {
	contracts.ChatStream
	llm *liveLLM
}

func (s *recordedStream) Next(ctx context.Context) (contracts.StreamEvent, error) {
	ev, err := s.ChatStream.Next(ctx)
	if ev.Kind == contracts.StreamEventToolCallDone && ev.ToolCall != nil {
		s.llm.mu.Lock()
		s.llm.calls = append(s.llm.calls, ev.ToolCall.Name)
		s.llm.mu.Unlock()
	}
	return ev, err
}

/*
liveDesk is an action desk whose agent answers with a real model. It
runs only when DIMMI_LIVE_LLM=1 and ANTHROPIC_API_KEY are set, because
every run spends real money; CI never sets them.
*/
func liveDesk(t *testing.T) (*refundDesk, *liveLLM) {
	t.Helper()
	if !testkit.Switch("DIMMI_LIVE_LLM") {
		t.Skip("live model tests run only with DIMMI_LIVE_LLM=1 and ANTHROPIC_API_KEY")
	}
	key := testkit.Env(t, "ANTHROPIC_API_KEY")
	r := newRefundDesk(t)
	live := &liveLLM{Driver: anthropic.New(anthropic.Options{APIKey: key})}
	r.d.TC.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return live, nil
	})
	r.d.TC.App.LLM.Flush()
	r.operator(`UPDATE ai_agents SET answer_model = $1`, liveModel)
	return r, live
}

/*
logRun writes what a live run did, for the person reading the test's
output.
*/
func (r *refundDesk) logRun(live *liveLLM, label string, run models.AiRun) {
	t := r.t
	t.Helper()
	var reply string
	for _, m := range r.load(*run.ConversationID, run.ID).agentReplies() {
		reply = str(m.Body)
	}
	t.Logf("%s: outcome=%s handoff=%s tools=%v", label, deref(run.Outcome), deref(run.HandoffReason), live.called())
	t.Logf("%s: trace=%s", label, string(run.ProcedureTrace))
	t.Logf("%s: last reply=%q", label, reply)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestLiveProcedures(t *testing.T) {
	t.Run("follows chips, waits for the lookup, and takes the branch code decided", func(t *testing.T) {
		r, live := liveDesk(t)
		r.shipping("delivered")
		r.api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 201, Body: []byte(`{"id":"re_1"}`)})
		conv, trigger := r.conversation("Hi, order 1042 arrived broken. Please refund me $15 (1500 cents).")
		r.signIn(conv)

		res := r.run(conv, trigger)
		run := r.load(conv, res.RunID).run
		r.logRun(live, "refund", run)

		assert.Contains(t, live.called(), "follow_procedure")
		require.Len(t, r.sentTo("GET", "/api/orders/1042"), 1, "the required lookup ran")
		refunds := r.sentTo("POST", "/api/orders/1042/refunds")
		if len(refunds) > 0 {
			assert.True(t, firstIndex(r, "GET") < firstIndex(r, "POST"), "a refund never runs before the lookup")
		}
		tr := trace(t, run)
		steps, _ := tr["steps"].([]any)
		branch, _ := steps[1].(map[string]any)
		assert.Equal(t, "then", branch["branch_taken"], "code took the delivered side")
	})

	t.Run("keeps following the procedure on the next message", func(t *testing.T) {
		r, live := liveDesk(t)
		r.shipping("shipped")
		conv, trigger := r.conversation("I want a refund for one of my orders.")
		r.signIn(conv)

		first := r.run(conv, trigger)
		r.logRun(live, "turn 1", r.load(conv, first.RunID).run)
		require.Equal(t, "active", r.procedureState(conv)[0].Status, "the procedure is kept for the next message")

		res := r.run(conv, r.add(conv, "contact", "message", "It's order 1042."))
		run := r.load(conv, res.RunID).run
		r.logRun(live, "turn 2", run)

		assert.True(t, followedProcedure(run.Flags), "the second turn follows it without asking again")
		require.Len(t, r.sentTo("GET", "/api/orders/1042"), 1)
		assert.Empty(t, r.sentTo("POST", "/api/orders/1042/refunds"), "the side not taken never refunds")
		tr := trace(t, run)
		steps, _ := tr["steps"].([]any)
		branch, _ := steps[1].(map[string]any)
		assert.Equal(t, "else", branch["branch_taken"])
	})

	t.Run("never holds a reply back for a conditional action", func(t *testing.T) {
		r, live := liveDesk(t)
		id := r.liveProcedure("Shipping questions", testkit.JSON{
			"description": "The customer asks about shipping: where we ship, how long it takes, or where an order is.",
			"steps": []testkit.JSON{{
				"id": "s1", "kind": "instruction",
				"text": "Only if they ask where a specific order is, check it with [[action:" + r.lookup + "]]. " +
					"Otherwise answer that we ship worldwide in 3 to 7 days.",
				"bindings": []testkit.JSON{{"kind": "action", "ref": r.lookup, "execution": "conditional"}},
			}},
			"handover": []string{},
		})
		conv, trigger := r.conversation("Do you ship to Canada, and how long does it take?")
		r.signIn(conv)

		res := r.run(conv, trigger)
		run := r.load(conv, res.RunID).run
		r.logRun(live, "conditional", run)

		assert.Contains(t, run.Flags, "procedure:"+mustDecode(t, r, id))
		assert.NotEqual(t, "procedure_guard", deref(run.HandoffReason))
		assert.NotContains(t, guards(trace(t, run)), "compliance_retry", "a conditional action is never forced")
		assert.Empty(t, r.sentTo("GET", "/api/orders/1042"))
	})

	t.Run("makes the model call a required action, or hands over", func(t *testing.T) {
		r, live := liveDesk(t)
		r.shipping("shipped")
		id := r.liveProcedure("Delivery complaints", testkit.JSON{
			"description": "The customer complains that an order is late.",
			"steps": []testkit.JSON{
				{"id": "s1", "kind": "instruction", "text": "Apologize for the delay in one sentence."},
				{"id": "s2", "kind": "instruction", "text": "Check order 1042 with [[action:" + r.lookup + "]]."},
				{"id": "s3", "kind": "instruction", "text": "Tell them when it arrives."},
			},
			"handover": []string{},
		})
		conv, trigger := r.conversation("My order is late again. This is so annoying.")
		r.signIn(conv)

		res := r.run(conv, trigger)
		run := r.load(conv, res.RunID).run
		r.logRun(live, "required", run)

		assert.Contains(t, run.Flags, "procedure:"+mustDecode(t, r, id))
		called := len(r.sentTo("GET", "/api/orders/1042")) == 1
		assert.True(t, called || deref(run.HandoffReason) == "procedure_guard",
			"the required lookup ran, or the guardrail handed the conversation over")
	})

	t.Run("holds the model to a required action when its guidance discourages tools", func(t *testing.T) {
		r, live := liveDesk(t)
		r.shipping("shipped")
		r.operator(`UPDATE ai_agents SET guidance = $1`,
			"Answer from what you already know in one short sentence. Avoid calling tools whenever you can.")
		r.liveProcedure("Late orders", testkit.JSON{
			"description": "The customer says an order is late.",
			"steps": []testkit.JSON{
				{"id": "s1", "kind": "instruction", "text": "Say sorry and that it usually arrives within a week."},
				{"id": "s2", "kind": "instruction", "text": "Then look up order 1042 with [[action:" + r.lookup +
					"]]."},
			},
			"handover": []string{},
		})
		conv, trigger := r.conversation("Order 1042 is late. When does it come?")
		r.signIn(conv)

		res := r.run(conv, trigger)
		run := r.load(conv, res.RunID).run
		r.logRun(live, "retry", run)

		called := len(r.sentTo("GET", "/api/orders/1042")) == 1
		assert.True(t, called || deref(run.HandoffReason) == "procedure_guard")
		t.Logf("retry: guard events %v", guards(trace(t, run)))
	})

	t.Run("simulates with mocked actions and never calls the shop", func(t *testing.T) {
		r, live := liveDesk(t)
		res := r.d.Owner.PostJSON("/api/v1/procedures/"+r.proc+"/simulate", testkit.JSON{
			"message": "Order 1042 arrived smashed, I'd like my $15 back.", "persona": signedInPersona(),
			"action_mocks": testkit.JSON{r.lookup: testkit.JSON{"body": testkit.JSON{"status": "delivered"}}},
		}).AssertOK()
		var body map[string]any
		res.Decode(&body)
		out, _ := json.Marshal(body["data"])
		t.Logf("simulation: tools=%v result=%s", live.called(), out)

		res.AssertJSONPath("data.matched", true).AssertJSONPath("data.action_calls.0.tool", "get_order")
		assert.Empty(t, r.api.Sent(), "no request reaches the shop in a simulation")
	})
}

/*
firstIndex is where the first request of a method sits among those the
shop received.
*/
func firstIndex(r *refundDesk, method string) int {
	for i, s := range r.api.Sent() {
		if strings.EqualFold(s.Method, method) {
			return i
		}
	}
	return -1
}

/*
mustDecode turns a procedure's public id into its row id text.
*/
func mustDecode(t *testing.T, r *refundDesk, public string) string {
	t.Helper()
	id, err := r.d.TC.App.IDs.Decode("prc", public)
	require.NoError(t, err)
	return id.String()
}
