package ai_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	decisionfake "github.com/dimmihq/dimmi/app/drivers/decision/fake"
)

/*
TestAiRunRobustness keeps the agent correct when models misbehave,
messages arrive in bursts, teammates step in and jobs retry.
*/
func TestAiRunRobustness(t *testing.T) {
	t.Parallel()

	t.Run("treats an unsure intent as a question", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		unsure := decide("spam", "en")
		unsure.Answers["intent"] = contracts.DecisionAnswer{Type: contracts.DecisionChoice, Choice: "spam", Confidence: 0.3}
		h.decider.Script(unsure)
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much does the Pro plan cost per seat?")
		res := h.run(conv, trigger)
		assert.Equal(t, "answered", res.Outcome)
		assert.Equal(t, "question", str(h.load(conv, res.RunID).run.Intent))
	})

	t.Run("falls back to a question when the decision model fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decisionfake.Step{Err: errors.New("decision model down")})
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much does the Pro plan cost per seat?")
		res := h.run(conv, trigger)
		assert.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "classifier_fallback")
		assert.Equal(t, []string{"classify", "retrieve", "generate"}, s.stepKinds())
		assert.Equal(t, "error", s.steps[0].Status)
	})

	t.Run("routes without a model when no decision model is configured", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.d.TC.App.AI.Decider = nil
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))
		h.llm.cheap.Script(summaryStep())

		conv, trigger := h.conversation("How much does the Pro plan cost per seat?")
		res := h.run(conv, trigger)
		assert.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "decider_unavailable")
		assert.Equal(t, []string{"retrieve", "generate"}, s.stepKinds())
		assert.Contains(t, sentText(h.llm.answer), "The workspace default language is English")

		next := h.add(conv, "contact", "message", "ok let me talk to a human")
		assert.Equal(t, "customer_requested", h.run(conv, next).HandoffReason)
		assert.Empty(t, h.decider.Sent())
	})

	t.Run("prices each decision from its input tokens", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("acknowledgement", "en"))

		conv, trigger := h.conversation("thanks")
		res := h.run(conv, trigger)
		s := h.load(conv, res.RunID)
		require.Equal(t, []string{"classify"}, s.stepKinds())
		assert.Equal(t, int64(84_000), s.steps[0].CostMicros)
		assert.Equal(t, "typesafe", str(s.steps[0].Provider))
		assert.Equal(t, int64(84_000), s.run.CostMicros)

		asked := h.decider.Sent()[0]
		assert.Contains(t, asked.State, "<customer_message>thanks</customer_message>")
		for _, name := range []string{"intent", "language", "wants_human", "frustrated", "injection_suspected"} {
			assert.Contains(t, asked.Questions, name)
		}
	})

	t.Run("searches with the previous message on a followup", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"), decide("followup", "en"))
		h.llm.answer.Script(
			reply("STATUS: answered\n\nSAML single sign-on is on the Business plan [1]."),
			reply("STATUS: answered\n\nIt works with Okta [1]."),
		)

		conv, trigger := h.conversation("Do you support SAML single sign-on?")
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)
		res := h.run(conv, h.add(conv, "contact", "message", "and with okta?"))
		require.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "Do you support SAML single sign-on?\n\nand with okta?", str(s.run.QueryRewritten))
	})

	t.Run("retries once when the answer ignores the output protocol", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(
			reply("Pro costs $12 per seat per month [1]."),
			reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."),
		)

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, []string{"classify", "retrieve", "generate", "generate"}, s.stepKinds())
		sent := h.llm.answer.Sent()
		require.Len(t, sent, 2)
		assert.Contains(t, sent[1].Messages[2].Parts[0].Text, "STATUS: answered")
	})

	t.Run("hands off when the answer ignores the protocol twice", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.cheap.Script(summaryStep())
		h.llm.answer.Script(reply("Sure! Pro is cheap."))

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "error", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "protocol_error", str(s.run.ErrorCode))
		assert.Equal(t, "handed_off", s.conv.AiState)
		for _, m := range s.agentReplies() {
			assert.NotContains(t, str(m.Body), "Pro is cheap")
		}
	})

	t.Run("never shows the model internal notes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, first := h.conversation("Hi there")
		h.add(conv, "member", "note", "NOTE-7731: customer owes us money, be careful")
		h.add(conv, "member", "message", "Hello! How can I help?")
		trigger := h.add(conv, "contact", "message", "How much is Pro per seat?")
		require.NotEqual(t, first, trigger)

		assert.Equal(t, "answered", h.run(conv, trigger).Outcome)
		sent := sentText(h.llm.answer, h.llm.cheap)
		assert.NotContains(t, sent, "NOTE-7731")
		assert.Contains(t, sent, `<turn role="teammate">Hello! How can I help?</turn>`)
		assert.Contains(t, sent, `<turn role="customer">Hi there</turn>`)
	})

	t.Run("discards the answer when a teammate replies while the model writes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro?")
		h.llm.hook = func() { h.add(conv, "member", "message", "Pro is $12 per seat, Ana.") }

		res := h.run(conv, trigger)
		assert.Equal(t, "superseded", res.Outcome)
		assert.Equal(t, "human_replied", res.SkipReason)

		s := h.load(conv, res.RunID)
		assert.Empty(t, s.agentReplies())
		assert.Equal(t, int32(0), s.conv.AiReplyCount)
	})

	t.Run("answers a burst of three messages once", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, first := h.conversation("hi")
		second := h.add(conv, "contact", "message", "I have a question")
		third := h.add(conv, "contact", "message", "how much is pro per seat?")

		assert.Equal(t, "debounced", h.run(conv, first).SkipReason)
		assert.Equal(t, "debounced", h.run(conv, second).SkipReason)
		res := h.run(conv, third)
		assert.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Len(t, s.runs, 1)
		assert.Len(t, s.agentReplies(), 1)
		assert.Contains(t, sentText(h.llm.answer),
			"<customer_message>hi\n\nI have a question\n\nhow much is pro per seat?</customer_message>")
	})

	t.Run("never answers twice when the job retries", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro?")
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)

		again := h.run(conv, trigger)
		assert.Equal(t, "skipped", again.Outcome)
		assert.Equal(t, "already_answered", again.SkipReason)

		s := h.load(conv, again.RunID)
		assert.Len(t, s.agentReplies(), 1)
		assert.Len(t, s.runs, 1)
		assert.Len(t, h.llm.answer.Sent(), 1)
	})

	t.Run("skips a trigger message that is gone", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, _ := h.conversation("How much is Pro?")

		missing := h.run(conv, ids.New())
		assert.Equal(t, "trigger_missing", missing.SkipReason)
		assert.Empty(t, h.load(conv, missing.RunID).runs)
	})

	t.Run("trims an answer cut off by the token limit to its last full sentence", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]. It adds unlimited proj"))
		h.llm.stop = contracts.StopReasonMaxTokens

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		body := str(h.load(conv, res.RunID).agentReplies()[0].Body)
		assert.Equal(t, "Pro costs $12 per seat per month.", body)
		assert.False(t, strings.Contains(body, "proj"))
	})

	t.Run("sends the agent's temperature and output cap", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("UPDATE ai_agents SET temperature = 0.2 WHERE workspace_id = $1", h.d.TC.Workspace.Workspace.ID)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro?")
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)

		req := h.llm.answer.Sent()[0]
		require.NotNil(t, req.Temperature)
		assert.InDelta(t, 0.2, *req.Temperature, 0.0001)
		assert.Equal(t, 600, req.MaxOutputTokens)
		assert.Positive(t, req.Timeout.Total)
	})
}
