package ai_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

/*
TestAiRunRobustness keeps the agent correct when models misbehave,
messages arrive in bursts, teammates step in and jobs retry.
*/
func TestAiRunRobustness(t *testing.T) {
	t.Parallel()

	t.Run("falls back to a question when the classifier answers nonsense", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.classifier.Script(fake.Step{Text: "I think this is a question about pricing."})
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much does the Pro plan cost per seat?")
		res := h.run(conv, trigger)
		assert.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "classifier_fallback")
		assert.Equal(t, "question", str(s.run.Intent))
		assert.Equal(t, []string{"classify", "classify", "retrieve", "generate"}, s.stepKinds())
	})

	t.Run("falls back to a question when the classifier fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.classifier.Script(fake.Step{Err: &contracts.LLMError{Kind: contracts.LLMErrorTimeout}})
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much does the Pro plan cost per seat?")
		assert.Equal(t, "answered", h.run(conv, trigger).Outcome)
	})

	t.Run("retries once when the answer ignores the output protocol", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"))
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
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"), summaryStep())
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
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, first := h.conversation("Hi there")
		h.add(conv, "member", "note", "NOTE-7731: customer owes us money, be careful")
		h.add(conv, "member", "message", "Hello! How can I help?")
		trigger := h.add(conv, "contact", "message", "How much is Pro per seat?")
		require.NotEqual(t, first, trigger)

		assert.Equal(t, "answered", h.run(conv, trigger).Outcome)
		sent := sentText(h.llm.answer, h.llm.classifier)
		assert.NotContains(t, sent, "NOTE-7731")
		assert.Contains(t, sent, `<turn role="teammate">Hello! How can I help?</turn>`)
		assert.Contains(t, sent, `<turn role="customer">Hi there</turn>`)
	})

	t.Run("discards the answer when a teammate replies while the model writes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"))
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
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"))
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
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"))
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
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"))
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
		h.llm.classifier.Script(classify("question", "en", "How much does the Pro plan cost per seat?"))
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
