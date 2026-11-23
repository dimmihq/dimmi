package ai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/contracts"
	decisionfake "github.com/dimmihq/dimmi/app/drivers/decision/fake"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

/*
systemOf joins the system prompt of one request to the model.
*/
func systemOf(req contracts.ChatRequest) string {
	var b strings.Builder
	for _, block := range req.System {
		b.WriteString(block.Text + "\n")
	}
	return b.String()
}

/*
TestReplyCheck covers the check every reply clears before it is sent: a
model reads it, and text never meant for the customer sends it back to
be written again rather than being cut out.
*/
func TestReplyCheck(t *testing.T) {
	t.Parallel()

	t.Run("sends a reply with a note to itself back to be written again", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, trigger := h.conversation("How much is Pro per seat?")
		h.llm.answer.Script(search("pro plan price per seat"),
			reply("Pro costs $12 per seat per month [1]. [billing cycle source needed]"),
			reply("Pro costs $12 per seat per month [1]."))
		h.llm.judge(map[string]bool{"not_for_customer": true})
		h.llm.onVerdict = func() { h.llm.judge(map[string]bool{}) }

		res := h.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "reply_regenerated")
		assert.NotContains(t, s.run.Flags, "reply_withheld")
		sent := h.llm.answer.Sent()
		require.Len(t, sent, 3, "the reply was written a second time")
		last := sent[2].Messages[len(sent[2].Messages)-1].Parts[0].Text
		assert.Contains(t, last, "That reply was not sent", "the agent is told why")
		assert.Equal(t, "Pro costs $12 per seat per month.", *s.agentReplies()[0].Body,
			"what the customer reads is the second reply, whole")
	})

	t.Run("sends the reply the agent wrote beside a tool call when it then adds nothing", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, trigger := h.conversation("How much is Pro per seat?")
		h.llm.answer.Script(search("pro plan price per seat"),
			fake.Step{
				Text:      "Pro costs $12 per seat per month [1].",
				ToolCalls: []contracts.ToolCall{call("triage_conversation", `{"title":"Pro price per seat"}`)},
				Usage:     contracts.LLMUsage{InputTokens: 1200, OutputTokens: 80, Reported: true},
			},
			reply(""))

		res := h.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome, "the reply was written; saying nothing more is not an unknown")
		s := h.load(conv, res.RunID)
		require.Len(t, s.agentReplies(), 1)
		assert.Equal(t, "Pro costs $12 per seat per month.", *s.agentReplies()[0].Body)
	})

	t.Run("hands over rather than send a reply that still carries it", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, trigger := h.conversation("How much is Pro per seat?")
		h.llm.answer.Script(search("pro plan price per seat"),
			reply("Pro costs $12 per seat per month [1]. [billing cycle source needed]"))
		h.llm.judge(map[string]bool{"not_for_customer": true})

		res := h.run(conv, trigger)

		assert.Equal(t, "handoff", res.Outcome)
		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "reply_withheld")
		for _, m := range s.agentReplies() {
			assert.NotContains(t, *m.Body, "source needed", "the note never reaches the customer")
		}
	})
}

/*
TestJudgmentFallback covers what happens when the decision model cannot
answer: it is asked again, then the workspace's answer model is asked
the same questions.
*/
func TestJudgmentFallback(t *testing.T) {
	t.Parallel()

	t.Run("asks the decision model twice, then the answer model", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		d, err := decisionfake.New("testing")
		require.NoError(t, err)
		d.Script(decisionfake.Step{Err: errors.New("decision model is down")})
		h.d.TC.App.AI.Decider = d
		conv, trigger := h.conversation("How much is Pro per seat?")
		h.llm.answer.Script(search("pro plan price per seat"), reply("Pro costs $12 per seat per month [1]."))

		res := h.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		assert.Len(t, d.Sent(), 2, "the decision model is asked again before giving up on it")
		require.Len(t, h.llm.judgments(), 1, "then the answer model judges the reply")
		assert.Contains(t, h.llm.judgments()[0].Messages[0].Parts[0].Text, "claims_handoff")
	})

	t.Run("does not ask the answer model when the decision model answers", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		d := h.decider()
		conv, trigger := h.conversation("How much is Pro per seat?")
		h.llm.answer.Script(search("pro plan price per seat"), reply("Pro costs $12 per seat per month [1]."))

		res := h.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		assert.Len(t, d.Sent(), 1)
		assert.Empty(t, h.llm.judgments())
	})
}

/*
TestStepJudgments covers how a procedure moves on what a model reads in
the conversation rather than on the agent's word or a step's wording.
*/
func TestStepJudgments(t *testing.T) {
	t.Parallel()

	stay := testkit.JSON{
		"description": "The customer wants to cancel.",
		"steps": []testkit.JSON{
			{"id": "why", "kind": "instruction", "text": "Find out what made them decide to cancel."},
			{
				"id": "still", "kind": "condition", "text": "The customer still wants to cancel",
				"then_steps": []testkit.JSON{
					{"id": "go", "kind": "instruction", "text": "Say the team will cancel it."},
				},
				"else_steps": []testkit.JSON{
					{"id": "stay", "kind": "instruction", "text": "Thank them for staying."},
				},
			},
		},
		"handover": []string{},
	}

	t.Run("moves past a step the customer answered before it was asked", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Cancel", stay)
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.judge(map[string]bool{"waits": true, "given": true, "clear": false})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))), reply("Do you still want to cancel?"))

		conv, trigger := h.conversation("Cancel my plan, it costs too much")
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)

		second := systemOf(h.llm.answer.Sent()[1])
		assert.Contains(t, second, "1. Find out what made them decide to cancel. [DONE]")
		assert.Contains(t, second, "Do not ask for it again")
	})

	t.Run("leaves a step the customer has not answered for the agent to ask", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Cancel", stay)
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.judge(map[string]bool{"waits": true, "given": false, "clear": false})
		h.llm.answer.Script(
			tools(call("follow_procedure", string(args))),
			tools(call("update_procedure_status", `{"status":"step_done","step":"1","still_missing":""}`)),
			reply("What made you decide to cancel?"),
		)

		conv, trigger := h.conversation("Cancel my plan")
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)

		sent := h.llm.answer.Sent()
		assert.Contains(t, systemOf(sent[1]), "1. Find out what made them decide to cancel. [ACTIVE]")
		refused := sent[2].Messages[len(sent[2].Messages)-1].Parts[0].Text
		assert.Contains(t, refused, "has not given what step 1 needs",
			"the agent's own report is refused when the conversation does not bear it out")
	})

	t.Run("decides a condition in plain words from the conversation and leaves the other side", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Cancel", stay)
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.judge(map[string]bool{"waits": true, "given": true, "clear": true, "holds": false})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))), reply("Thank you for staying."))

		conv, trigger := h.conversation("I wanted to cancel over the price, but I will stay after all")
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)

		second := systemOf(h.llm.answer.Sent()[1])
		assert.Contains(t, second, "Say the team will cancel it. [NOT TAKEN]")
		assert.NotContains(t, second, "Thank them for staying. [NOT TAKEN]")
	})
}

/*
TestSimulationJudge covers a saved simulation's check written as a
sentence: the decision model judges it, not the agent that wrote the
replies.
*/
func TestSimulationJudge(t *testing.T) {
	t.Parallel()

	sentence := func(t *testing.T, met float64) (*testkit.Response, *decisionfake.Decider, *harness) {
		t.Helper()
		h := newHarness(t, nil)
		d := h.decider()
		d.Script(decisionfake.Step{Answers: map[string]contracts.DecisionAnswer{
			"met": {Type: contracts.DecisionNoul, Noul: met},
		}})
		if met < 0 {
			down := errors.New("model is down")
			d.Script(decisionfake.Step{Err: down})
			h.llm.judgeErr = down
		}
		id := h.liveProcedure("Cancel", testkit.JSON{
			"description": "The customer wants to cancel.",
			"steps":       []testkit.JSON{{"id": "why", "kind": "instruction", "text": "Say the team will cancel it."}},
			"handover":    []string{},
		})
		base := "/api/v1/procedures/" + id + "/test-cases"
		h.d.Owner.PostJSON(base, testkit.JSON{
			"name": "Cancel", "messages": []testkit.JSON{{"text": "Cancel my plan"}},
			"assertions": []testkit.JSON{{"type": "llm_rubric", "rubric": "dimmi says the team will cancel it"}},
		}).AssertCreated()
		h.llm.answer.Script(reply("The team will cancel your plan."))

		return h.d.Owner.PostJSON(base+"/run", nil).AssertOK(), d, h
	}

	t.Run("passes a sentence check the decision model finds met", func(t *testing.T) {
		t.Parallel()
		res, d, h := sentence(t, 0.93)
		res.AssertJSONPath("data.0.last_result.passed", true).
			AssertJSONPath("data.0.last_result.assertion_results.0.message", "Met.")
		asked := d.Sent()[len(d.Sent())-1]
		assert.Contains(t, asked.Questions["met"].Instructions, "dimmi says the team will cancel it")
		assert.Contains(t, asked.State, "CUSTOMER: Cancel my plan\nADA: The team will cancel your plan.")
		assert.Empty(t, h.llm.judgments(), "the agent's own model does not grade its work")
	})

	t.Run("fails a sentence check the decision model finds unmet", func(t *testing.T) {
		t.Parallel()
		res, _, _ := sentence(t, 0.08)
		res.AssertJSONPath("data.0.last_result.passed", false).
			AssertJSONPath("data.0.last_result.assertion_results.0.message", "Not met.")
	})

	t.Run("marks a sentence check the decision model only leans toward as unsure, without failing the run",
		func(t *testing.T) {
			t.Parallel()
			res, _, _ := sentence(t, 0.58)
			res.AssertJSONPath("data.0.last_result.passed", true).
				AssertJSONPath("data.0.last_result.assertion_results.0.passed", false).
				AssertJSONPath("data.0.last_result.assertion_results.0.unsure", true)
			assert.Contains(t, res.String("data.0.last_result.assertion_results.0.message"), "could not tell")
		})

	t.Run("passes a sentence check from sixty in a hundred", func(t *testing.T) {
		t.Parallel()
		res, _, _ := sentence(t, 0.61)
		res.AssertJSONPath("data.0.last_result.passed", true).
			AssertJSONPath("data.0.last_result.assertion_results.0.message", "Met.")
	})

	t.Run("fails the simulation when the decision model cannot judge a sentence check twice", func(t *testing.T) {
		t.Parallel()
		res, d, h := sentence(t, -1)
		res.AssertJSONPath("data.0.last_result.passed", false).
			AssertJSONPath("data.0.last_result.assertion_results.0.passed", false).
			AssertJSONPath("data.0.last_result.assertion_results.0.message", "This check could not be judged.")
		assert.Contains(t, res.String("data.0.last_result.error"), "The simulation failed")
		assert.Contains(t, res.String("data.0.last_result.error"), "Try again.")
		assert.GreaterOrEqual(t, len(d.Sent()), 2, "the decision model was asked again")
		for _, asked := range h.llm.judgments() {
			assert.NotContains(t, string(asked.Schema.Schema), `"met"`, "the answer model never judges a check")
		}
	})
}

/*
TestReadableReplies covers the layout a reply gets before it is sent: a
dense paragraph is broken up, and not one word of it changes.
*/
func TestReadableReplies(t *testing.T) {
	t.Parallel()

	h := newHarness(t, nil)
	conv, trigger := h.conversation("How do refunds work?")
	h.llm.answer.Script(search("refunds"), reply("Refunds go to your balance [1]. The carrier confirms first. "+
		"USPS takes 14 to 21 days. Other carriers take 7 days. You can ask for it on your card."))

	res := h.run(conv, trigger)

	require.Equal(t, "answered", res.Outcome)
	body := *h.load(conv, res.RunID).agentReplies()[0].Body
	assert.Equal(t, "Refunds go to your balance. The carrier confirms first. USPS takes 14 to 21 days.\n\n"+
		"Other carriers take 7 days. You can ask for it on your card.", body)
}

/*
TestSimulatedCustomer covers a saved simulation run against a customer
who answers whatever dimmi asks now, from the chat it was saved from.
*/
func TestSimulatedCustomer(t *testing.T) {
	t.Parallel()

	saved := func(t *testing.T, body testkit.JSON) (*harness, string) {
		t.Helper()
		h := newHarness(t, nil)
		id := h.liveProcedure("Cancel", testkit.JSON{
			"description": "The customer wants to cancel.",
			"steps":       []testkit.JSON{{"id": "why", "kind": "instruction", "text": "Ask why they want to cancel."}},
			"handover":    []string{},
		})
		base := "/api/v1/procedures/" + id + "/test-cases"
		body["name"] = "Cancel"
		body["assertions"] = []testkit.JSON{{"type": "handed_off", "handed_off": false}}
		h.d.Owner.PostJSON(base, body).AssertCreated()

		return h, base + "/run"
	}

	t.Run("answers a question the saved chat did not expect, and says when it had to", func(t *testing.T) {
		t.Parallel()
		h, run := saved(t, testkit.JSON{"messages": []testkit.JSON{{"text": "Cancel my plan"}}})
		h.player.moves = []pipeline.CustomerMove{
			{Message: "I don't have an order number.", Unknown: true},
			{Done: true},
		}
		h.llm.answer.Script(reply("What is your order number?"), reply("No problem, I'll cancel it."))

		res := h.d.Owner.PostJSON(run, nil).AssertOK()

		res.AssertJSONPath("data.0.last_result.turns.#", float64(2)).
			AssertJSONPath("data.0.last_result.turns.1.message", "I don't have an order number.").
			AssertJSONPath("data.0.last_result.turns.1.improvised", true).
			AssertJSONPath("data.0.last_result.passed", true)
		require.NotEmpty(t, h.player.asked)
		assert.Equal(t, "Cancel my plan", h.player.asked[0].Saved[0].Text, "the customer reads the saved chat")
		assert.Equal(t, "What is your order number?", h.player.asked[0].Run[1].Text, "and this run's chat")
	})

	t.Run("fails the run without a verdict when the customer cannot be played", func(t *testing.T) {
		t.Parallel()
		h, run := saved(t, testkit.JSON{"messages": []testkit.JSON{{"text": "Cancel my plan"}, {"text": "Too dear"}}})
		h.player.fail = errors.New("every host is down")
		h.llm.answer.Script(reply("Why do you want to cancel?"))

		res := h.d.Owner.PostJSON(run, nil).AssertOK()

		res.AssertJSONPath("data.0.last_result.passed", false)
		assert.Contains(t, res.String("data.0.last_result.error"), "did not answer")
		assert.Contains(t, res.String("data.0.last_result.error"), "Run it again")
	})

	t.Run("runs on the date the simulation keeps", func(t *testing.T) {
		t.Parallel()
		h, run := saved(t, testkit.JSON{
			"messages": []testkit.JSON{{"text": "Cancel my plan"}}, "simulated_at": "2026-03-14T10:00:00Z",
		})
		h.llm.answer.Script(reply("Why do you want to cancel?"))

		h.d.Owner.PostJSON(run, nil).AssertOK()

		sent := h.llm.answer.Sent()
		require.NotEmpty(t, sent)
		first := sent[0].Messages[len(sent[0].Messages)-1].Parts[0].Text
		assert.Contains(t, first, "Saturday 14 March 2026", "the agent sees the simulation's day, not today")
	})

	t.Run("does not run against a working copy with problems to fix", func(t *testing.T) {
		t.Parallel()
		h, run := saved(t, testkit.JSON{"messages": []testkit.JSON{{"text": "Cancel my plan"}}})
		id, _, _ := strings.Cut(strings.TrimPrefix(run, "/api/v1/procedures/"), "/")
		h.d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": testkit.JSON{
			"description": "The customer wants to cancel.",
			"steps": []testkit.JSON{{
				"id": "rule", "kind": "branch", "text": "Big company",
				"condition": testkit.JSON{"mode": "all", "rules": []testkit.JSON{
					{"field": "company.attr.seats", "operator": "gte", "value": ""},
				}},
				"then_steps": []testkit.JSON{}, "else_steps": []testkit.JSON{},
			}},
			"handover": []string{},
		}}).AssertOK()

		res := h.d.Owner.PostJSON(run, nil).AssertOK()

		res.AssertJSONPath("data.0.last_result.passed", false)
		assert.Contains(t, res.String("data.0.last_result.error"), "working copy has problems to fix first")
		assert.Empty(t, h.llm.answer.Sent(), "nothing is spent on a run that could only mislead")
	})
}
