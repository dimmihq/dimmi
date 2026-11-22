package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
activeState reads the state a simulated turn handed back.
*/
func activeState(r *testkit.Response) any {
	var body struct {
		Data struct {
			ActiveState any `json:"active_state"`
		} `json:"data"`
	}
	r.Decode(&body)
	return body.Data.ActiveState
}

/*
signedInPersona is a made-up customer the shop's actions can bind.
*/
func signedInPersona() testkit.JSON {
	return testkit.JSON{"verified": true, "external_id": "user_42", "name": "Ana", "channel": "widget"}
}

func TestAiProcedureSimulator(t *testing.T) {
	t.Parallel()

	t.Run("simulates turns with mocked actions and hands the state back", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		base := "/api/v1/procedures/" + r.proc + "/simulate"
		mocks := testkit.JSON{
			r.lookup: testkit.JSON{"status": 200, "body": testkit.JSON{"status": "delivered", "eta": "Monday"}},
		}
		r.llm.judge(map[string]bool{"asks": true})
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			tools(call("get_order", `{"order_id":"1042"}`)),
			reply("It was delivered on Monday. How much should I refund?"),
		)

		first := r.d.Owner.PostJSON(base, testkit.JSON{
			"message": "My order 1042 arrived broken", "persona": signedInPersona(), "action_mocks": mocks,
		}).AssertOK()

		first.AssertJSONPath("data.object", "procedure_simulation").AssertJSONPath("data.matched", true).
			AssertJSONPath("data.outcome", "answered").AssertJSONPath("data.action_calls.0.tool", "get_order").
			AssertJSONPath("data.action_calls.0.succeeded", true).
			AssertJSONPath("data.active_state.status", "active").
			AssertJSONPath("data.active_state.called_action_ids.0", r.lookup).
			AssertJSONPath("data.active_state.step_outputs.get_order.status", "delivered").
			AssertJSONPath("data.procedure_trace.steps.1.branch_taken", "then")
		assert.Empty(t, r.api.Sent(), "no request leaves dimmi")
		assert.Contains(t, first.String("data.run_id"), "air_")

		r.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":1500}`)), reply("Refunded."))
		second := r.d.Owner.PostJSON(base, testkit.JSON{
			"message": "$15 please", "persona": signedInPersona(), "action_mocks": mocks,
			"history": []testkit.JSON{
				{"role": "customer", "text": "My order 1042 arrived broken"},
				{"role": "agent", "text": first.String("data.reply")},
			},
			"active_state": activeState(first),
		}).AssertOK()

		second.AssertJSONPath("data.action_calls.0.tool", "refund_order").
			AssertJSONPath("data.active_state.status", "completed")
		assert.Contains(t, sentText(r.llm.answer), "<active_procedure>")
		assert.Empty(t, r.api.Sent())
	})

	t.Run("waits for the tester to confirm a write, then runs it", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		conn := a.connection()
		lookup := a.action(conn, orderLookup())
		confirmRefund := a.action(conn, refund("customer", "none"))
		id := a.liveProcedure("Refunds", refundProcedure(lookup, confirmRefund))
		base := "/api/v1/procedures/" + id + "/simulate"
		mocks := testkit.JSON{lookup: testkit.JSON{"body": testkit.JSON{"status": "delivered"}}}
		args := `{"procedure_id":"` + id + `"}`
		a.llm.answer.Script(
			tools(call("follow_procedure", args)), tools(call("get_order", `{"order_id":"1042"}`)),
			tools(call("refund_order", `{"order_id":"1042","amount_cents":900}`)), reply("Please confirm the refund."),
		)

		waiting := a.d.Owner.PostJSON(base, testkit.JSON{
			"message": "Refund order 1042", "persona": signedInPersona(), "action_mocks": mocks,
		}).AssertOK()
		waiting.AssertJSONPath("data.active_state.pending.0.action_id", confirmRefund).
			AssertJSONPath("data.active_state.pending.0.state", "awaiting_confirmation")

		a.llm.answer.Script(reply("Your refund went through."))
		done := a.d.Owner.PostJSON(base, testkit.JSON{
			"persona": signedInPersona(), "action_mocks": mocks, "active_state": activeState(waiting),
			"confirmed_action_ids": []string{confirmRefund},
		}).AssertOK()

		done.AssertJSONPath("data.action_calls.0.tool", "refund_order").
			AssertJSONPath("data.action_calls.0.succeeded", true).
			AssertJSONPath("data.active_state.pending.#", float64(0))
		assert.Contains(t, sentText(a.llm.answer), "was confirmed and ran")
		assert.Empty(t, a.api.Sent())
	})

	t.Run("runs saved tests and keeps how each went", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		base := "/api/v1/procedures/" + r.proc + "/test-cases"
		test := r.d.Owner.PostJSON(base, testkit.JSON{
			"name": "Delivered order", "persona": signedInPersona(),
			"messages":     []testkit.JSON{{"text": "Order 1042 arrived broken, refund $15"}},
			"action_mocks": testkit.JSON{r.lookup: testkit.JSON{"body": testkit.JSON{"status": "delivered"}}},
			"assertions": []testkit.JSON{
				{"type": "procedure_matched"},
				{"type": "action_order", "action": "get_order", "before": "refund_order"},
				{"type": "action_called", "action": "refund_order", "args": testkit.JSON{"order_id": "1042"}},
				{"type": "branch_taken", "step_id": "s2", "branch": "then"},
				{"type": "reply_contains", "text": "refunded"},
				{"type": "handed_off", "handed_off": false},
			},
		}).AssertCreated().String("data.id")
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())), tools(call("get_order", `{"order_id":"1042"}`)),
			tools(call("refund_order", `{"order_id":"1042","amount_cents":1500}`)), reply("You are refunded $15."),
		)

		res := r.d.Owner.PostJSON(base+"/run", testkit.JSON{"test_case_ids": []string{test}}).AssertOK()

		res.AssertJSONPath("data.0.last_result.passed", true).
			AssertJSONPath("data.0.last_result.assertion_results.#", float64(6)).
			AssertJSONPath("data.0.last_result.turns.0.reply", "You are refunded $15.")
		r.d.Owner.GetJSON(base+"/"+test).AssertOK().AssertJSONPath("data.last_result.passed", true)
		assert.Empty(t, r.api.Sent())

		r.llm.answer.Script(tools(call("follow_procedure", r.follow())), reply("Sorry, no refunds."))
		r.d.Owner.PostJSON(base+"/run", nil).AssertOK().AssertJSONPath("data.0.last_result.passed", false)
	})

	t.Run("refuses a turn with nothing to answer", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		r.d.Owner.PostJSON("/api/v1/procedures/"+r.proc+"/simulate", testkit.JSON{}).AssertStatus(422).
			AssertJSONPath("code", "simulation_invalid")
		r.d.Owner.PostJSON("/api/v1/procedures/"+r.proc+"/simulate", testkit.JSON{"message": "x", "target": "nope"}).
			AssertStatus(422)
		require.Empty(t, r.llm.answer.Sent())
	})
}

/*
refreshFacts brings the report facts up to date.
*/
func (h *harness) refreshFacts() {
	h.t.Helper()
	far := h.d.TC.App.Clock.Now().AddDate(1, 0, 0)
	for range 10 {
		n, err := h.d.TC.App.Actions.ReportMaintenance.Drain(h.t.Context(), far)
		require.NoError(h.t, err)
		if n == 0 {
			return
		}
	}
}

func TestAiProcedureScorecard(t *testing.T) {
	t.Parallel()

	t.Run("counts resolutions the way the dimmi agent report does", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Sign in", testkit.JSON{
			"description": "The customer cannot sign in.", "handover": []string{},
			"steps": []testkit.JSON{{"id": "s1", "kind": "instruction", "text": "Send them the reset link."}},
		})
		args := `{"procedure_id":"` + id + `"}`
		for i, text := range []string{"I can't sign in", "Locked out of my account"} {
			h.llm.answer.Script(tools(call("follow_procedure", args)), reply("Here is the reset link."))
			conv, trigger := h.conversation(text)
			h.run(conv, trigger)
			if i == 0 {
				path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conv)
				h.d.Owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK()
			}
		}
		h.refreshFacts()

		card := h.d.Owner.GetJSON("/api/v1/procedures/" + id + "/metrics?days=7").AssertOK()
		card.AssertJSONPath("data.matched_conversations", float64(2)).AssertJSONPath("data.ai_involved", float64(2)).
			AssertJSONPath("data.ai_resolved", float64(1)).AssertJSONPath("data.ai_resolution_rate", 0.5).
			AssertJSONPath("data.steps.0.step_id", "s1")

		var report struct {
			Data struct {
				Metrics []struct {
					Key   string   `json:"key"`
					Value *float64 `json:"value"`
				} `json:"metrics"`
			} `json:"data"`
		}
		h.d.Owner.GetJSON("/api/v1/reports/ai").AssertOK().Decode(&report)
		found := false
		for _, m := range report.Data.Metrics {
			if m.Key == "ai_resolution" && m.Value != nil {
				found = true
				assert.InDelta(t, 50.0, *m.Value, 0.01, "the scorecard and the report agree")
			}
		}
		assert.True(t, found)
		h.d.Owner.GetJSON("/api/v1/reports/procedures").AssertOK().AssertJSONPath("data.0.procedure_id", id).
			AssertJSONPath("data.0.ai_resolved", float64(1))
	})
}
