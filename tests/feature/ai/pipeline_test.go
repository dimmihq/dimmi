package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
TestAiPipeline answers customers end to end on the real application,
with scripted models and the seed articles as knowledge.
*/
func TestAiPipeline(t *testing.T) {
	t.Parallel()

	t.Run("answers a question with citations from the sources", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nThe Pro plan costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro per seat?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "completed", s.run.Status)
		assert.Equal(t, "v2", s.run.PromptVersion)
		assert.Equal(t, []string{"classify", "retrieve", "generate"}, s.stepKinds())
		assert.Positive(t, s.run.CostMicros)
		assert.Positive(t, s.run.OutputTokens)
		assert.Equal(t, int32(1), s.conv.AiReplyCount)
		assert.Equal(t, "active", s.conv.AiState)

		replies := s.agentReplies()
		require.Len(t, replies, 1)
		assert.Equal(t, "The Pro plan costs $12 per seat per month.", str(replies[0].Body))
		assert.True(t, replies[0].Substantive)
		citations, ok := aiMeta(t, replies[0])["sources"].([]any)
		require.True(t, ok)
		require.Len(t, citations, 1)
		first, _ := citations[0].(map[string]any)
		assert.Equal(t, "https://help.lovelace.test/plans-and-pricing", first["url"])

		sent := sentText(h.llm.answer)
		assert.Contains(t, sent, "You are Ada, the AI support assistant for Lovelace")
		assert.Contains(t, sent, `<source id="1" title="Plans and pricing"`)
		assert.Contains(t, sent, `<contact name="Ana"`)
		assert.Contains(t, sent, "<customer_message>How much is Pro per seat?</customer_message>")
		assert.Contains(t, sent, "Reply per the output protocol: the STATUS line, a blank line, then the reply.")
		assert.NotContains(t, sent, "{{")
	})

	t.Run("answers the covered part of a partly covered question", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nSAML single sign-on is on the Business plan [1]. " +
			"The documentation does not cover SCIM, so a teammate can confirm that part."))

		conv, trigger := h.conversation("Do you have SAML SSO with SCIM?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		replies := h.load(conv, res.RunID).agentReplies()
		require.Len(t, replies, 1)
		assert.Contains(t, str(replies[0].Body), "does not cover SCIM")
	})

	t.Run("hands off when nothing answers the question and the agent is set to hand off", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.UnknownBehavior = "handoff" })
		h.decider.Script(decide("question", "en"))
		h.llm.cheap.Script(summaryStep())

		conv, trigger := h.conversation("What's the weather in Paris tomorrow?")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "unknown", res.HandoffReason)
		assert.Empty(t, h.llm.answer.Sent())

		s := h.load(conv, res.RunID)
		assert.Equal(t, "handed_off", s.conv.AiState)
		replies := s.agentReplies()
		require.Len(t, replies, 1)
		assert.Contains(t, str(replies[0].Body), "couldn't find that in our documentation")
		assert.False(t, replies[0].Substantive)
		require.Len(t, s.notes(), 1)
		assert.Contains(t, str(s.notes()[0].Body), "Wants:")
	})

	t.Run("offers a person when the model cannot answer", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MinSimilarity = database.NumericFloat(0.1) })
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: unknown\n\nThe documentation doesn't say whether PayPal works [1]. " +
			"A teammate can help with that."))

		conv, trigger := h.conversation("Can I pay for the Business plan with PayPal?")
		res := h.run(conv, trigger)
		assert.Equal(t, "unknown", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "active", s.conv.AiState)
		replies := s.agentReplies()
		require.Len(t, replies, 1)
		assert.Equal(t, "The documentation doesn't say whether PayPal works. A teammate can help with that.",
			str(replies[0].Body))
		assert.Equal(t, []any{"handoff"}, aiMeta(t, replies[0])["actions"])
		assert.Equal(t, int32(0), s.conv.AiReplyCount)
	})

	t.Run("hands off a second unknown in a row", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))

		conv, trigger := h.conversation("What's the weather in Paris?")
		require.Equal(t, "unknown", h.run(conv, trigger).Outcome)

		next := h.add(conv, "contact", "message", "And in Rome?")
		res := h.run(conv, next)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "unknown_streak", res.HandoffReason)
	})

	t.Run("treats an uncited answer as unknown when citations are required", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro is very affordable."))

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "unknown", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "uncited")
		assert.Contains(t, str(s.agentReplies()[0].Body), "couldn't find that in our documentation")
	})

	t.Run("passes the customer's language through", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MinSimilarity = database.NumericFloat(0.1) })
		h.decider.Script(decide("question", "de"))
		h.llm.answer.Script(reply("STATUS: answered\n\nDer Pro-Plan kostet 12 $ pro Nutzer und Monat [1]."))

		conv, trigger := h.conversation("Was kostet der Pro Plan pro Seat?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "Der Pro-Plan kostet 12 $ pro Nutzer und Monat.", str(s.agentReplies()[0].Body))
		assert.Equal(t, "de", str(s.run.Language))
		assert.Equal(t, "de", str(s.conv.AiLanguage))
		assert.Contains(t, sentText(h.llm.answer), "The workspace default language is German")
	})
}
