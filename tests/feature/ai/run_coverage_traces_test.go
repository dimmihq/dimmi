package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
adaRuns are the agent runs the tracker counts (services/span.rs
agent_summary: gen_ai_operation_type = 'agent') of the desk's agent,
Ada.
*/
func adaRuns(spans []tests.TrackedSpan) []tests.TrackedSpan {
	var out []tests.TrackedSpan
	for _, s := range spansOf(spans, "agent") {
		if s.Attr("gen_ai.agent.name") == "Ada" {
			out = append(out, s)
		}
	}
	return out
}

func TestRunCoverageTraces(t *testing.T) {
	t.Parallel()

	simulate := func(t *testing.T, steps ...fake.Step) []tests.TrackedSpan {
		t.Helper()
		tracker := tests.NewTracker(t)
		h := newHarness(t, nil, tracker.Option())
		path := agentPath(t, h.d)
		id := h.d.Owner.PostJSON(path+"/simulations", testkit.JSON{
			"name":       "Quotes the price",
			"messages":   []testkit.JSON{{"text": "How much is it?"}},
			"assertions": []testkit.JSON{{"type": "reply_contains", "text": "$29"}},
		}).AssertCreated().String("data.id")
		h.llm.answer.Script(steps...)

		h.d.Owner.PostJSON(path+"/simulations/run", testkit.JSON{"simulation_ids": []string{id}}).AssertOK()
		return h.d.TC.TrackedSpans(tracker)
	}

	t.Run("a simulation is a named agent run, told apart from a customer's", func(t *testing.T) {
		t.Parallel()
		spans := simulate(t, search("price"), reply("Growth is $29 a seat each month."))

		runs := adaRuns(spans)
		require.NotEmpty(t, runs)
		run := runs[0]
		assert.Equal(t, "simulation", run.Attr("dimmi.run.kind"))
		assert.Equal(t, true, run.Data["dimmi.simulation"])
		assert.NotEmpty(t, run.Attr("dimmi.run_id"), "the simulated turn's run is named")
		assert.Empty(t, run.Status)
		assertOneChat(t, runs)
		var used, calls int
		for _, s := range spans {
			if agentOf(spans, s) != "Ada" || s.SpanID == run.SpanID {
				continue
			}
			switch s.Kind() {
			case "tool":
				used++
			case "ai_client":
				calls++
			}
		}
		assert.Positive(t, used, "its tool calls sit beneath it")
		assert.Positive(t, calls, "its model calls sit beneath it")
	})

	t.Run("a simulation the model fails is a failed run", func(t *testing.T) {
		t.Parallel()
		spans := simulate(t, fake.Step{Err: &contracts.LLMError{Kind: contracts.LLMErrorAuth, HTTPStatus: 401}})

		runs := adaRuns(spans)
		require.NotEmpty(t, runs)
		assert.Equal(t, "internal_error", runs[0].Status)
	})

	t.Run("a customer's turn says so", func(t *testing.T) {
		t.Parallel()
		tracker := tests.NewTracker(t)
		h := newHarness(t, nil, tracker.Option())
		h.llm.answer.Script(reply("Growth is $29."))
		conv, trigger := h.conversation("How much is it?")

		h.run(conv, trigger)
		runs := adaRuns(h.d.TC.TrackedSpans(tracker))

		require.Len(t, runs, 1)
		assert.Equal(t, "conversation", runs[0].Attr("dimmi.run.kind"))
		assert.Equal(t, false, runs[0].Data["dimmi.simulation"])
	})

	t.Run("an action's follow-up is its own named run in the conversation", func(t *testing.T) {
		t.Parallel()
		tracker := tests.NewTracker(t)
		a := newActionDesk(t, nil, tracker.Option())
		a.api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_77"}`)})
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Order 1042 arrived broken, refund me 20 dollars.")
		a.signIn(conv)
		a.llm.answer.Script(
			tools(call("refund_order", `{"order_id":"1042","amount_cents":2000}`)),
			reply("I've asked the team to approve a refund of $20.00."),
		)
		a.run(conv, trigger)
		member, _ := a.d.Join(t, enums.RoleMember)
		member.PatchJSON("/api/v1/ai/action-calls/"+publicCall(a.onlyCall(conv)),
			testkit.JSON{"status": "approved"}).AssertOK()
		a.llm.answer.Script(reply("Your $20.00 refund was approved."))
		a.drain()

		var followUp []tests.TrackedSpan
		for _, run := range adaRuns(a.d.TC.TrackedSpans(tracker)) {
			if run.Attr("dimmi.run.kind") == "action_follow_up" {
				followUp = append(followUp, run)
			}
		}
		require.Len(t, followUp, 1)
		assert.Equal(t, conv.String(), followUp[0].Attr("gen_ai.conversation.id"))
		assert.Empty(t, followUp[0].Status)
	})
}

/*
assertOneChat checks that the runs of one simulated chat are grouped
like a conversation: each carries the same conversation id.
*/
func assertOneChat(t *testing.T, runs []tests.TrackedSpan) {
	t.Helper()
	chat := runs[0].Attr("gen_ai.conversation.id")
	assert.NotEmpty(t, chat, "a simulated chat is grouped like a conversation")
	for _, run := range runs {
		assert.Equal(t, chat, run.Attr("gen_ai.conversation.id"), "its turns share one chat")
	}
}
