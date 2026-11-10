package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TestAiTurn answers customers end to end on the real application, with
a scripted model calling the agent's tools over the seed articles.
*/
func TestAiTurn(t *testing.T) {
	t.Parallel()

	t.Run("answers after one search with citations", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("pro plan price per seat"),
			reply("The Pro plan costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro per seat?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "completed", s.run.Status)
		assert.Equal(t, "v2", s.run.PromptVersion)
		assert.Equal(t, []string{"generate", "tool_call", "generate"}, s.stepKinds())
		var undated int
		require.NoError(t, h.d.TC.Operator().QueryRow(t.Context(),
			"SELECT count(*) FROM ai_run_steps WHERE run_id = $1 AND created_at < now() - interval '1 year'",
			res.RunID).Scan(&undated))
		assert.Zero(t, undated, "every step is stamped with when it was written")
		assert.Positive(t, s.run.CostMicros)
		assert.Equal(t, "pro plan price per seat", str(s.run.QueryRewritten))
		assert.Equal(t, int32(1), s.conv.AiReplyCount)
		assert.Equal(t, "active", s.conv.AiState)

		replies := s.agentReplies()
		require.Len(t, replies, 1)
		assert.Equal(t, "The Pro plan costs $12 per seat per month.", str(replies[0].Body))
		assert.True(t, replies[0].Substantive)
		meta := aiMeta(t, replies[0])
		assert.Equal(t, true, meta["generated"])
		sources, ok := meta["sources"].([]any)
		require.True(t, ok)
		require.Len(t, sources, 1)
		first, _ := sources[0].(map[string]any)
		assert.Equal(t, "https://help.lovelace.test/plans-and-pricing", first["url"])

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 2)
		assert.Contains(t, sent[0].System[0].Text, "Ada")
		assert.Contains(t, sent[0].System[0].Text, "Lovelace")
		assert.NotContains(t, sent[0].System[0].Text, "{{")
		assert.Contains(t, toolNames(sent[0]), "search_knowledge")
		assert.Contains(t, toolNames(sent[0]), "handoff")
		assert.Contains(t, toolNames(sent[0]), "triage_conversation")
		first0 := sent[0].Messages[0].Parts[0].Text
		assert.Contains(t, first0, `<contact name="Ana"></contact>`)
		assert.Contains(t, first0, "<customer_message>How much is Pro per seat?</customer_message>")
		results := sent[1].Messages[len(sent[1].Messages)-1]
		assert.Equal(t, contracts.ChatRoleTool, results.Role)
		assert.Contains(t, results.Parts[0].Text, `"id":1`)
	})

	t.Run("numbers results across two searches", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("SAML single sign-on"), search("refund annual plan"),
			reply("SSO is on Business [1], and annual plans are refundable within 30 days [6]."))

		conv, trigger := h.conversation("Do you have SSO, and can I get a refund?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, []string{"generate", "tool_call", "generate", "tool_call", "generate"}, s.stepKinds())
		sources, _ := aiMeta(t, s.agentReplies()[0])["sources"].([]any)
		urls := []any{}
		for _, src := range sources {
			m, _ := src.(map[string]any)
			urls = append(urls, m["url"])
		}
		assert.Contains(t, urls, "https://help.lovelace.test/sso-saml")
		second := h.llm.answer.Sent()[2]
		assert.NotContains(t, second.Messages[len(second.Messages)-1].Parts[0].Text, `"id":1,`)
	})

	t.Run("says it does not know when nothing is found", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("weather Paris"),
			reply("I couldn't find anything about the weather [3]; a teammate can help."))

		conv, trigger := h.conversation("What's the weather in Paris tomorrow?")
		res := h.run(conv, trigger)
		assert.Equal(t, "unknown", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Contains(t, h.llm.answer.Sent()[1].Messages[2].Parts[0].Text, `{"results":[]}`)
		replies := s.agentReplies()
		require.Len(t, replies, 1)
		assert.Equal(t, "I couldn't find anything about the weather; a teammate can help.", str(replies[0].Body))
		assert.False(t, replies[0].Substantive)
		assert.Equal(t, []any{"handoff"}, aiMeta(t, replies[0])["actions"])
		assert.Equal(t, "active", s.conv.AiState)
	})

	t.Run("hands off a second unknown in a row", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("weather"), reply("I don't know that."))

		conv, trigger := h.conversation("What's the weather in Paris?")
		require.Equal(t, "unknown", h.run(conv, trigger).Outcome)
		h.llm.answer.Script(search("weather Rome"), reply("I don't know that either."))
		res := h.run(conv, h.add(conv, "contact", "message", "And in Rome?"))
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "unknown_streak", res.HandoffReason)
	})

	t.Run("hands off at once when the agent is set to hand off unknowns", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.UnknownBehavior = "handoff" })
		h.llm.answer.Script(search("weather"), reply(""))

		conv, trigger := h.conversation("What's the weather in Paris?")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "unknown", res.HandoffReason)

		s := h.load(conv, res.RunID)
		body := str(s.agentReplies()[0].Body)
		assert.Contains(t, body, "couldn't find that in our documentation")
		assert.Contains(t, body, "bringing in a teammate")
		assert.Contains(t, sentText(h.llm.answer), "call handoff with reason unknown at once")
		require.Len(t, s.notes(), 1)
	})

	t.Run("replies to small talk without searching", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Hi Ana, what can I help you with?"))

		conv, trigger := h.conversation("hello")
		res := h.run(conv, trigger)
		assert.Equal(t, "greeted", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.False(t, s.agentReplies()[0].Substantive)
		assert.Nil(t, aiMeta(t, s.agentReplies()[0])["actions"])
		assert.Equal(t, int32(0), s.conv.AiReplyCount)
	})

	t.Run("tells the model the workspace language and channel", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.DefaultLanguage = "de" })
		h.llm.answer.Script(search("pro plan cost"), reply("Der Pro-Plan kostet 12 $ pro Nutzer und Monat [1]."))

		conv, trigger := h.conversation("Was kostet der Pro Plan?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "Der Pro-Plan kostet 12 $ pro Nutzer und Monat.", str(s.agentReplies()[0].Body))
		system := h.llm.answer.Sent()[0].System[0].Text
		assert.Contains(t, system, "German")
		assert.Contains(t, system, "chat")
	})
}

/*
toolNames lists the tools a request offered.
*/
func toolNames(req contracts.ChatRequest) []string {
	out := make([]string, len(req.Tools))
	for i, t := range req.Tools {
		out[i] = t.Name
	}
	return out
}
