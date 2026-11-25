package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
refundDesk is an action desk with the shop's order lookup and refund
published, and a refund procedure that follows them.
*/
type refundDesk struct {
	*actionDesk
	lookup string
	refund string
	proc   string
}

/*
refundProcedure looks the order up, then decides in code: delivered, it
refunds after the lookup; still on its way, it only says so.
*/
func refundProcedure(lookup, refundID string) testkit.JSON {
	return testkit.JSON{
		"description": "The customer wants a refund for an order.",
		"steps": []testkit.JSON{
			{"id": "s1", "kind": "instruction", "text": "Look the order up with [[action:" + lookup + "]]."},
			{
				"id": "s2", "kind": "branch", "text": "Delivered",
				"condition": testkit.JSON{"mode": "all", "rules": []testkit.JSON{
					{"field": "action.get_order.status", "operator": "eq", "value": "delivered"},
				}},
				"then_steps": []testkit.JSON{{
					"id": "s3", "kind": "instruction", "text": "Refund it with [[action:" + refundID + "]].",
					"enforcement": testkit.JSON{"requires": []string{"s1"}},
				}},
				"else_steps": []testkit.JSON{{
					"id": "s4", "kind": "instruction",
					"text": "Say it is on its way and when it arrives, and ask whether that works for them.",
				}},
			},
		},
		"handover": []string{},
	}
}

/*
newRefundDesk publishes the actions and the procedure.
*/
func newRefundDesk(t *testing.T) *refundDesk {
	t.Helper()
	a := newActionDesk(t, nil)
	conn := a.connection()
	r := &refundDesk{actionDesk: a}
	r.lookup = a.action(conn, orderLookup())
	r.refund = a.action(conn, refund("none", "none"))
	r.proc = a.liveProcedure("Refunds", refundProcedure(r.lookup, r.refund))
	return r
}

/*
follow is the call that follows the refund procedure.
*/
func (r *refundDesk) follow() string {
	args, _ := json.Marshal(map[string]string{"procedure_id": r.proc})
	return string(args)
}

/*
shipping stubs the shop's answer for order 1042.
*/
func (r *refundDesk) shipping(status string) {
	r.api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{
		Status: 200, Body: []byte(`{"status":"` + status + `","eta":"Thursday"}`),
	})
}

/*
trace reads a run's procedure trace.
*/
func trace(t *testing.T, run models.AiRun) map[string]any {
	t.Helper()
	require.NotEmpty(t, run.ProcedureTrace, "the run keeps its procedure trace")
	var out map[string]any
	require.NoError(t, json.Unmarshal(run.ProcedureTrace, &out))
	return out
}

/*
guards lists the types of a trace's guardrail events.
*/
func guards(tr map[string]any) []string {
	var out []string
	list, _ := tr["guard_events"].([]any)
	for _, g := range list {
		if m, ok := g.(map[string]any); ok {
			kind, _ := m["type"].(string)
			out = append(out, kind)
		}
	}
	return out
}

/*
procedureState reads the conversation's procedure state rows, newest
first.
*/
func (r *refundDesk) procedureState(conv ids.UUID) []models.ConversationProcedureState {
	r.t.Helper()
	var out []models.ConversationProcedureState
	rows, err := r.d.TC.Operator().Query(r.t.Context(), `SELECT status, called_action_ids, completed_step_ids
		FROM conversation_procedure_states WHERE conversation_id = $1 ORDER BY started_at DESC, id DESC`, conv)
	require.NoError(r.t, err)
	defer rows.Close()
	for rows.Next() {
		var s models.ConversationProcedureState
		require.NoError(r.t, rows.Scan(&s.Status, &s.CalledActionIds, &s.CompletedStepIds))
		out = append(out, s)
	}
	require.NoError(r.t, rows.Err())
	return out
}

/*
lastSystem joins the system blocks of the model's nth call.
*/
func (r *refundDesk) system(n int) string {
	sent := r.llm.answer.Sent()
	if n >= len(sent) {
		return ""
	}
	var b strings.Builder
	for _, s := range sent[n].System {
		b.WriteString(s.Text + "\n")
	}
	return b.String()
}

/*
toolResult is the text of the last tool result the model read on its
nth call.
*/
func (r *refundDesk) toolResult(n int) string {
	msgs := r.llm.answer.Sent()[n].Messages
	return msgs[len(msgs)-1].Parts[0].Text
}

func TestAiProcedureGuardrails(t *testing.T) {
	t.Parallel()

	t.Run("blocks a refund before the lookup and follows the branch code decided", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		r.shipping("delivered")
		r.api.Stub(
			"POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 201, Body: []byte(`{"id":"re_1"}`)},
		)
		conv, trigger := r.conversation("I want a refund for order 1042, it arrived broken")
		r.signIn(conv)
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			tools(call("refund_order", `{"order_id":"1042","amount_cents":1500}`)),
			tools(call("get_order", `{"order_id":"1042"}`)),
			tools(call("refund_order", `{"order_id":"1042","amount_cents":1500}`)),
			reply("Your refund of $15 is on its way."),
		)

		res := r.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		followed := r.toolResult(1)
		assert.Contains(t, followed, "get_order tool (required)")
		assert.Contains(t, followed, "[LOCKED: call get_order first to unlock the next steps]")
		assert.NotContains(t, followed, "refund_order", "the locked branch shows neither side")

		blocked := r.toolResult(2)
		assert.Contains(t, blocked, "procedure_prerequisite_unmet")
		assert.Contains(t, blocked, "Cannot call refund_order yet. You must call get_order first (Step 1).")

		assert.Contains(t, r.system(3), "is true, so follow these steps")
		assert.Contains(t, r.system(3), "2.1. Refund it with the refund_order tool (required). [ACTIVE]")
		assert.NotContains(t, r.system(3), "on its way and when it arrives", "the side not taken is never shown")
		require.Len(t, r.sentTo("POST", "/api/orders/1042/refunds"), 1, "the refund runs once, after the lookup")

		tr := trace(t, r.load(conv, res.RunID).run)
		assert.Equal(t, r.proc, tr["procedure_id"])
		assert.Equal(t, float64(1), tr["version"])
		assert.Equal(t, []string{"prerequisite_blocked"}, guards(tr))
		steps, _ := tr["steps"].([]any)
		branch, _ := steps[1].(map[string]any)
		assert.Equal(t, "then", branch["branch_taken"])
		assert.Equal(t, true, branch["deterministic"])

		states := r.procedureState(conv)
		require.Len(t, states, 1)
		assert.Equal(t, "completed", states[0].Status, "every required action on its path ran")
		assert.ElementsMatch(t, []string{r.lookup, r.refund}, states[0].CalledActionIds)
	})

	t.Run("sends a reply back three times when a required action is skipped, then hands over", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		conv, trigger := r.conversation("Refund my order 1042 please")
		r.signIn(conv)
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			reply("Sure, I will refund it."), reply("Okay, done."), reply("It is refunded."), reply("All set."),
		)

		res := r.run(conv, trigger)

		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "procedure_guard", res.HandoffReason)
		sent := r.llm.answer.Sent()
		require.Len(t, sent, 5, "the first reply and three retries")
		assert.Contains(t, r.toolResult(2), "Reply not sent. The active procedure requires calling get_order (Step 1)")
		assert.Contains(t, r.toolResult(3), "which takes these required parameters: order_id")
		assert.Contains(t, r.toolResult(4), "handed to a teammate automatically")
		s := r.load(conv, res.RunID)
		assert.Equal(
			t,
			[]string{"compliance_retry", "compliance_retry", "compliance_retry", "compliance_failed_handoff"},
			guards(trace(t, s.run)),
		)
		assert.Contains(t, s.run.Flags, "procedure_guard")
		require.NotEmpty(t, s.notes())
		assert.Contains(t, str(s.notes()[0].Body), "step 1 requires get_order")
		assert.Equal(t, "handed_off", r.procedureState(conv)[0].Status)
		assert.Empty(t, r.api.Sent(), "nothing ran")
	})

	t.Run("lets the agent comply on a retry", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		r.shipping("shipped")
		conv, trigger := r.conversation("Refund my order 1042 please")
		r.signIn(conv)
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			reply("Sure, I will refund it."),
			tools(call("get_order", `{"order_id":"1042"}`)),
			reply("It is still on its way and arrives Thursday."),
		)

		res := r.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		tr := trace(t, r.load(conv, res.RunID).run)
		assert.Equal(t, []string{"compliance_retry"}, guards(tr))
		assert.Equal(t, float64(1), tr["retries"])
		steps, _ := tr["steps"].([]any)
		branch, _ := steps[1].(map[string]any)
		assert.Equal(t, "else", branch["branch_taken"], "the refund on the side not taken never holds the reply back")
		assert.Empty(t, r.sentTo("POST", "/api/orders/1042/refunds"))
	})

	t.Run("lets the agent ask for what a required action needs", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		conv, trigger := r.conversation("I want a refund")
		r.signIn(conv)
		r.llm.judge(map[string]bool{"asks": true})
		r.llm.answer.Script(tools(call("follow_procedure", r.follow())), reply("Which order number is it?"))

		res := r.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		assert.Len(t, r.llm.answer.Sent(), 2, "no retry while the agent asks the customer")
		assert.Equal(t, []string{"awaiting_input"}, guards(trace(t, r.load(conv, res.RunID).run)))
	})

	t.Run("never holds a reply back for a conditional action", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		lookup := a.action(a.connection(), orderLookup())
		id := a.liveProcedure("Order status", testkit.JSON{
			"description": "The customer asks where an order is.",
			"steps": []testkit.JSON{{
				"id": "s1", "kind": "instruction",
				"text":     "If they give a number, check it with [[action:" + lookup + "]].",
				"bindings": []testkit.JSON{{"kind": "action", "ref": lookup, "execution": "conditional"}},
			}},
			"handover": []string{},
		})
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		conv, trigger := a.conversation("Do you ship to Canada")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("follow_procedure", string(args))), reply("Yes, we ship to Canada."))

		res := a.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		assert.Len(t, a.llm.answer.Sent(), 2)
		assert.Contains(t, sentText(a.llm.answer), "get_order tool (only when it applies)")
		assert.Empty(t, guards(trace(t, a.load(conv, res.RunID).run)))
	})
}

func TestAiProcedureState(t *testing.T) {
	t.Parallel()

	t.Run("keeps following the procedure on the next message until it is completed", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		r.shipping("delivered")
		r.api.Stub(
			"POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 201, Body: []byte(`{"id":"re_1"}`)},
		)
		conv, trigger := r.conversation("My order 1042 arrived broken")
		r.signIn(conv)
		r.llm.judge(map[string]bool{"asks": true})
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			tools(call("get_order", `{"order_id":"1042"}`)),
			reply("It was delivered. How much should I refund?"),
		)
		r.run(conv, trigger)

		second := len(r.llm.answer.Sent())
		r.llm.answer.Script(
			tools(call("refund_order", `{"order_id":"1042","amount_cents":1500}`)),
			reply("Done: $15 is on its way back to you."),
		)
		res := r.run(conv, r.add(conv, "contact", "message", "The full $15 please"))

		assert.Equal(t, "answered", res.Outcome)
		first := r.system(second)
		assert.Contains(t, first, "<active_procedure>", "the procedure comes back without asking for it")
		assert.Contains(t, first, "1. Look the order up with the get_order tool (required). [DONE]")
		assert.Contains(t, first, "\"status\":\"delivered\"", "with what its actions returned")
		var offered []string
		for _, tool := range r.llm.answer.Sent()[second].Tools {
			offered = append(offered, tool.Name)
		}
		assert.Contains(t, offered, "update_procedure_status")
		assert.Contains(t, offered, "refund_order")
		require.Len(t, r.sentTo("POST", "/api/orders/1042/refunds"), 1)
		assert.True(t, followedProcedure(r.load(conv, res.RunID).run.Flags), "the run counts as following it")
		states := r.procedureState(conv)
		require.Len(t, states, 1)
		assert.Equal(t, "completed", states[0].Status, "every required action ran, so it is done")

		r.llm.answer.Script(reply("Anything else?"))
		third := len(r.llm.answer.Sent())
		r.run(conv, r.add(conv, "contact", "message", "thanks"))
		assert.NotContains(t, r.system(third), "<active_procedure>", "a completed procedure is not followed again")
	})

	t.Run("stays on a step that talks to the customer until the agent says it is done", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		r.shipping("shipped")
		conv, trigger := r.conversation("Where is my refund for order 1042?")
		r.signIn(conv)
		r.llm.judge(map[string]bool{"waits": true, "given": false})
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			tools(call("get_order", `{"order_id":"1042"}`)),
			tools(call("update_procedure_status", `{"status":"step_done","step":"2.1"}`)),
			reply("It is on its way and arrives on Friday. Does that work for you?"),
		)

		res := r.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		states := r.procedureState(conv)
		require.Len(t, states, 1)
		assert.Equal(t, "active", states[0].Status, "a step still follows the last action, so it is not over")
		assert.NotContains(t, states[0].CompletedStepIds, "s4", "the customer has not replied to the step yet")
		sent := r.llm.answer.Sent()
		assert.Contains(t, r.toolResult(len(sent)-1), "has not given what step 2.1 needs yet")
		assert.Contains(t, r.system(len(sent)-1), "one at a time", "it is told to follow the steps as a conversation")

		r.llm.answer.Script(reply("Friday the 14th, by courier. Will someone be home to sign for it?"))
		r.run(conv, r.add(conv, "contact", "message", "Which Friday do you mean?"))
		states = r.procedureState(conv)
		require.Len(t, states, 1)
		assert.Equal(t, "active", states[0].Status, "a step may take several exchanges")
		assert.NotContains(t, states[0].CompletedStepIds, "s4", "the agent has not said the step is done")

		r.llm.judge(map[string]bool{"waits": true, "given": true})
		r.llm.answer.Script(
			tools(call("update_procedure_status", `{"status":"step_done","step":"2.1"}`)),
			reply("Great. Anything else?"),
		)
		r.run(conv, r.add(conv, "contact", "message", "Yes, Friday is fine"))

		states = r.procedureState(conv)
		require.Len(t, states, 1)
		assert.Contains(t, states[0].CompletedStepIds, "s4", "their reply let the step be finished")
	})

	t.Run("ends the procedure when the customer moves on", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		conv, trigger := r.conversation("I want a refund")
		r.signIn(conv)
		r.llm.judge(map[string]bool{"moved_on": true})
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			tools(call("update_procedure_status", `{"status":"exited_topic_changed","reason":"asks about shipping"}`)),
			reply("We ship worldwide."),
		)

		r.run(conv, trigger)

		assert.Equal(t, "exited", r.procedureState(conv)[0].Status)
		assert.Contains(t, r.toolResult(2), `"status":"exited"`)
	})

	t.Run("stops following the procedure once a teammate replies", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		conv, trigger := r.conversation("I want a refund")
		r.signIn(conv)
		r.llm.judge(map[string]bool{"asks": true})
		r.llm.answer.Script(tools(call("follow_procedure", r.follow())), reply("Which order number is it?"))
		r.run(conv, trigger)
		require.Equal(t, "active", r.procedureState(conv)[0].Status)

		r.add(conv, "member", "message", "Hi, I'm taking this one.")
		r.llm.answer.Script(reply("Sure."))
		next := len(r.llm.answer.Sent())
		r.run(conv, r.add(conv, "contact", "message", "ok"))

		assert.Equal(t, "handed_off", r.procedureState(conv)[0].Status)
		if next < len(r.llm.answer.Sent()) {
			assert.NotContains(t, r.system(next), "<active_procedure>")
		}
	})
}

func TestAiProcedureScope(t *testing.T) {
	t.Parallel()

	t.Run("offers only the procedures scoped to the conversation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		step := []testkit.JSON{{"id": "s1", "kind": "instruction", "text": "Ask for the details."}}
		email := h.liveProcedure("Email only", testkit.JSON{
			"description": "Email refunds.", "steps": step, "handover": []string{},
			"scope": testkit.JSON{"channels": []string{"email"}},
		})
		reusable := h.liveProcedure("Verify identity", testkit.JSON{
			"description": "Verify who they are.", "steps": step, "handover": []string{},
			"scope": testkit.JSON{"reusable_only": true},
		})
		otherInbox := h.liveProcedure("Other inbox", testkit.JSON{
			"description": "Another inbox.", "steps": step, "handover": []string{},
			"scope": testkit.JSON{"inbox_ids": []string{"inbox_01m6dqs9m0f18t4m32my65sjsf"}},
		})
		open := h.liveProcedure(
			"Anyone",
			testkit.JSON{"description": "Anything.", "steps": step, "handover": []string{}},
		)
		h.llm.answer.Script(reply("Hello!"))

		conv, trigger := h.conversation("hi")
		h.run(conv, trigger)

		var offered string
		for _, tool := range h.llm.answer.Sent()[0].Tools {
			if tool.Name == "follow_procedure" {
				offered = tool.Description
			}
		}
		assert.Contains(t, offered, open)
		assert.NotContains(t, offered, email)
		assert.NotContains(t, offered, reusable)
		assert.NotContains(t, offered, otherInbox)
	})
}
