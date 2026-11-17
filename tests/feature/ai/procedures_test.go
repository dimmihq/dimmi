package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	decisionfake "github.com/dimmihq/dimmi/app/drivers/decision/fake"
)

/*
liveProcedure publishes a procedure and returns its public id.
*/
func (h *harness) liveProcedure(name string, def testkit.JSON) string {
	h.t.Helper()
	res := h.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": name}).AssertCreated()
	id := res.String("data.id")
	h.d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": def}).AssertOK()
	h.d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertOK()

	return id
}

/*
decider gives the agent a scripted decision model and returns it.
*/
func (h *harness) decider() *decisionfake.Decider {
	h.t.Helper()
	d, err := decisionfake.New(string(foundation.EnvTesting))
	require.NoError(h.t, err)
	h.d.TC.App.AI.Decider = d

	return d
}

/*
fits scripts how likely each procedure is to fit the request.
*/
func fits(scores map[string]float64) decisionfake.Step {
	answers := map[string]contracts.DecisionAnswer{}
	for id, score := range scores {
		answers[id] = contracts.DecisionAnswer{Type: contracts.DecisionNoul, Noul: score}
	}

	return decisionfake.Step{Answers: answers, Usage: contracts.DecisionUsage{InputTokens: 500}}
}

func TestAiProcedures(t *testing.T) {
	t.Parallel()

	signIn := testkit.JSON{
		"description": "The customer cannot sign in.",
		"steps": []testkit.JSON{
			{"id": "s1", "kind": "instruction", "text": "Ask which email address they sign in with."},
		},
		"handover": []string{"They lost access to the mailbox."},
	}

	t.Run("offers live procedures and follows the one the request matches", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Send a sign-in code", signIn)
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			reply("Which email address do you sign in with?"))

		conv, trigger := h.conversation("I can't sign in to my account")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		first := h.llm.answer.Sent()[0]
		var offered string
		for _, tool := range first.Tools {
			if tool.Name == "follow_procedure" {
				offered = tool.Description
			}
		}
		assert.Contains(t, offered, id+": Send a sign-in code. Use when: The customer cannot sign in.")
		assert.Contains(t, sentText(h.llm.answer), "1. Ask which email address they sign in with.")
		assert.Contains(t, sentText(h.llm.answer), "They lost access to the mailbox.")

		s := h.load(conv, res.RunID)
		assert.True(t, followedProcedure(s.run.Flags))
		h.d.Owner.GetJSON("/api/v1/procedures").AssertOK().AssertJSONPath("data.0.uses", float64(1))
	})

	t.Run("keeps the procedure list in view when tools load on request", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withToolLayer()
		id := h.liveProcedure("Send a sign-in code", signIn)
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			reply("Which email address do you sign in with?"))

		conv, trigger := h.conversation("I can't sign in to my account")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		first := h.llm.answer.Sent()[0]
		var offered string
		for _, tool := range first.Tools {
			if tool.Name == "follow_procedure" {
				offered = tool.Description
			}
		}
		assert.Contains(t, offered, id+": Send a sign-in code. Use when: The customer cannot sign in.",
			"the agent sees every procedure and when it applies without loading anything")
		assert.Contains(t, sentText(h.llm.answer), "1. Ask which email address they sign in with.",
			"and follows it by name, not through call_tool")
		assert.True(t, followedProcedure(h.load(conv, res.RunID).run.Flags))
	})

	t.Run("offers no procedure tool when none is live", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": "Draft only"}).AssertCreated()
		h.llm.answer.Script(reply("Hello! How can I help?"))

		conv, trigger := h.conversation("hi")
		h.run(conv, trigger)
		for _, tool := range h.llm.answer.Sent()[0].Tools {
			assert.NotEqual(t, "follow_procedure", tool.Name)
		}
	})
}

func TestAiProcedureCheck(t *testing.T) {
	t.Parallel()

	signIn := testkit.JSON{
		"description": "The customer cannot sign in.",
		"steps": []testkit.JSON{
			{"id": "s1", "kind": "instruction", "text": "Ask which email address they sign in with."},
		},
	}
	cancel := testkit.JSON{
		"description": "The customer asks to cancel their plan.",
		"steps": []testkit.JSON{
			{"id": "s1", "kind": "instruction", "text": "Ask what made them decide to leave."},
		},
	}

	t.Run("hands over a procedure the decision model finds fitting", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Send a sign-in code", signIn)
		d := h.decider().Script(fits(map[string]float64{id: 0.9}))
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			reply("Which email address do you sign in with?"))

		conv, trigger := h.conversation("I can't sign in to my account")
		res := h.run(conv, trigger)

		require.Len(t, d.Sent(), 1)
		assert.Contains(t, d.Sent()[0].State, "CUSTOMER: I can't sign in to my account")
		assert.Equal(t, "The customer cannot sign in.", d.Sent()[0].Questions[id].True)
		assert.Contains(t, sentText(h.llm.answer), "1. Ask which email address they sign in with.")
		s := h.load(conv, res.RunID)
		assert.True(t, followedProcedure(s.run.Flags))
		assert.Contains(t, s.stepKinds(), "verify", "the check is recorded as a step of the run")
	})

	t.Run("holds back a procedure that does not fit, then hands it over when asked again", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Customer wants to cancel", cancel)
		d := h.decider().Script(fits(map[string]float64{id: 0.02}))
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			tools(call("follow_procedure", string(args))), reply("What made you decide to leave?"))

		conv, trigger := h.conversation("How do I set a 30 day return window?")
		res := h.run(conv, trigger)

		second := h.llm.answer.Sent()[1]
		held := second.Messages[len(second.Messages)-1].Parts[0].Text
		assert.Contains(t, held, "The steps were not loaded.")
		assert.Contains(t, held, "call this again with the same procedure_id")
		assert.NotContains(t, held, "Ask what made them decide to leave.")

		assert.Contains(t, sentText(h.llm.answer), "1. Ask what made them decide to leave.",
			"asking again gets the steps")
		assert.Len(t, d.Sent(), 1, "and is not checked a second time")
		assert.True(t, followedProcedure(h.load(conv, res.RunID).run.Flags))
	})

	t.Run("does not count a held-back procedure as followed", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Customer wants to cancel", cancel)
		h.decider().Script(fits(map[string]float64{id: 0.02}))
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))), reply("I'm not sure about that."))

		conv, trigger := h.conversation("How do I set a 30 day return window?")
		res := h.run(conv, trigger)

		assert.False(t, followedProcedure(h.load(conv, res.RunID).run.Flags))
		assert.NotContains(t, sentText(h.llm.answer), "Ask what made them decide to leave.")
	})

	t.Run("hands over the procedure that fits in place of the one asked for", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		wrong := h.liveProcedure("Customer wants to cancel", cancel)
		right := h.liveProcedure("Send a sign-in code", signIn)
		h.decider().Script(fits(map[string]float64{wrong: 0.01, right: 0.92}))
		args, _ := json.Marshal(map[string]string{"procedure_id": wrong})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			reply("Which email address do you sign in with?"))

		conv, trigger := h.conversation("I can't sign in to my account")
		res := h.run(conv, trigger)

		sent := sentText(h.llm.answer)
		assert.Contains(t, sent, "fits \\\"Send a sign-in code\\\" instead")
		assert.Contains(t, sent, "1. Ask which email address they sign in with.")
		assert.NotContains(t, sent, "Ask what made them decide to leave.")
		assert.Len(t, h.load(conv, res.RunID).run.Flags, 1, "only the procedure handed over is counted")
		list := h.d.Owner.GetJSON("/api/v1/procedures").AssertOK()
		for _, i := range []string{"0", "1"} {
			uses := float64(0)
			if list.String("data."+i+".id") == right {
				uses = 1
			}
			list.AssertJSONPath("data."+i+".uses", uses)
		}
	})

	t.Run("asks the decision model once when the agent asks for two procedures", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		first := h.liveProcedure("Customer wants to cancel", cancel)
		second := h.liveProcedure("Send a sign-in code", signIn)
		d := h.decider().Script(fits(map[string]float64{first: 0.8, second: 0.8}))
		one, _ := json.Marshal(map[string]string{"procedure_id": first})
		two, _ := json.Marshal(map[string]string{"procedure_id": second})
		h.llm.answer.Script(tools(call("follow_procedure", string(one))),
			tools(call("follow_procedure", string(two))), reply("Which email address do you sign in with?"))

		conv, trigger := h.conversation("I can't sign in, and I want to cancel")
		res := h.run(conv, trigger)

		assert.Len(t, d.Sent(), 1)
		assert.Len(t, h.load(conv, res.RunID).run.Flags, 2)
	})

	t.Run("does not check a procedure the conversation is already following", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Send a sign-in code", signIn)
		d := h.decider().Script(fits(map[string]float64{id: 0.9}), fits(map[string]float64{id: 0.01}))
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			reply("Which email address do you sign in with?"),
			tools(call("follow_procedure", string(args))), reply("Thanks, a code is on its way."))

		conv, trigger := h.conversation("I can't sign in to my account")
		h.run(conv, trigger)
		res := h.run(conv, h.add(conv, "contact", "message", "ok"))

		assert.Len(t, d.Sent(), 1, "a short follow-up does not put the procedure in question")
		assert.True(t, followedProcedure(h.load(conv, res.RunID).run.Flags))
	})

	t.Run("hands the procedure over when the decision model fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Send a sign-in code", signIn)
		h.decider().Script(decisionfake.Step{Err: assert.AnError})
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			reply("Which email address do you sign in with?"))

		conv, trigger := h.conversation("I can't sign in to my account")
		res := h.run(conv, trigger)

		require.Equal(t, "answered", res.Outcome)
		assert.Contains(t, sentText(h.llm.answer), "1. Ask which email address they sign in with.")
	})
}

/*
followedProcedure reports whether the flags name a procedure the run
followed.
*/
func followedProcedure(flags []string) bool {
	for _, f := range flags {
		if strings.HasPrefix(f, "procedure:") {
			return true
		}
	}

	return false
}
