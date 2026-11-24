package ai_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
TestAiSearchAhead searches the knowledge with the customer's first
message before the agent's first model call, and with nothing else.
*/
func TestAiSearchAhead(t *testing.T) {
	t.Parallel()

	t.Run("puts the search and its result in front of the agent, which answers in one call", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("The Pro plan costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro per seat?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 1, "one model call writes the answer")
		msgs := sent[0].Messages
		require.GreaterOrEqual(t, len(msgs), 3)
		searched, found := msgs[len(msgs)-2], msgs[len(msgs)-1]
		require.Len(t, searched.ToolCalls, 1)
		assert.Equal(t, contracts.ChatRoleAssistant, searched.Role)
		assert.Equal(t, "search_knowledge", searched.ToolCalls[0].Name)
		assert.JSONEq(t, `{"question":"How much is Pro per seat?"}`, string(searched.ToolCalls[0].Arguments),
			"the search is the customer's own words")
		assert.Equal(t, contracts.ChatRoleTool, found.Role)
		assert.Equal(t, searched.ToolCalls[0].ID, found.ToolCallID)
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(found.Parts[0].Text), &result))
		assert.NotEmpty(t, result["results"])
		assert.Contains(t, result["note"], "search again yourself", "the agent is told it may look further")
		assert.NotContains(t, msgs[0].Parts[0].Text, "<hidden_details>", "nothing was hidden, so nothing is said")

		s := h.load(conv, res.RunID)
		assert.Equal(t, []string{"tool_call", "generate", "verify"}, s.stepKinds())
		assert.Contains(t, string(s.steps[0].Response), `ahead\":true`, "the trace says who searched")
		assert.Nil(t, s.run.QueryRewritten, "it is not a query the agent wrote")
		assert.NotEmpty(t, aiMeta(t, s.agentReplies()[0])["sources"], "what it found can be cited")
	})

	t.Run("leaves a greeting unsearched", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Hi! What can I help you with?"))

		conv, trigger := h.conversation("Hello, good day")
		res := h.run(conv, trigger)
		assert.Equal(t, "greeted", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.NotContains(t, s.stepKinds(), "tool_call")
		assert.Len(t, h.llm.answer.Sent()[0].Messages, 1)
	})

	t.Run("adds nothing when the search finds nothing, so the agent looks for itself", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("What would you like to know?"))

		conv, trigger := h.conversation("I have two questions for you today.")
		res := h.run(conv, trigger)
		assert.Equal(t, "greeted", res.Outcome, "a reply that used no search is not an agent that found nothing")

		for _, m := range h.llm.answer.Sent()[0].Messages {
			assert.Empty(t, m.ToolCalls)
			assert.NotEqual(t, contracts.ChatRoleTool, m.Role)
		}
	})

	t.Run("searches only the first message of a conversation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("The Pro plan costs $12 per seat per month [1]."),
			reply("Is that for the annual plan?"))

		conv, trigger := h.conversation("How much is Pro per seat?")
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)
		next := h.add(conv, "contact", "message", "And can I get a refund on an annual plan?")
		res := h.run(conv, next)

		s := h.load(conv, res.RunID)
		assert.NotContains(t, s.stepKinds(), "tool_call", "a later message leans on what came before it")
	})

	t.Run("does not use up the searches the agent may make", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(search("a"), search("b"), search("c"), search("d"),
			reply("The Pro plan costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro per seat?")
		res := h.run(conv, trigger)

		s := h.load(conv, res.RunID)
		for _, step := range s.steps {
			assert.NotContains(t, string(step.Response), "search limit", "all four of the agent's own searches ran")
		}
	})
}
