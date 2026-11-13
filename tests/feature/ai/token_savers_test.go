package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAiTokenSavers(t *testing.T) {
	t.Parallel()

	t.Run("searches with the customer's message before the first call, compactly", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withTokenSavers()
		h.llm.answer.Script(reply("It costs per seat [1]."))

		conv, trigger := h.conversation("What is the pro plan price per seat?")
		res := h.run(conv, trigger)

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 1, "one model call answers")
		msgs := sent[0].Messages
		require.GreaterOrEqual(t, len(msgs), 3)
		call := msgs[len(msgs)-2]
		require.Len(t, call.ToolCalls, 1)
		assert.Equal(t, "search_knowledge", call.ToolCalls[0].Name)
		result := msgs[len(msgs)-1].Parts[0].Text
		assert.NotContains(t, result, `"results"`, "results are written compactly")
		assert.Contains(t, result, "[1] ")
		s := h.load(conv, res.RunID)
		var searches int
		for _, st := range s.steps {
			if st.Kind == "tool_call" && str(st.Model) == "search_knowledge" {
				searches++
			}
		}
		assert.Equal(t, 1, searches)
	})

	t.Run("does not search a thank-you", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withTokenSavers()
		h.llm.answer.Script(reply("You're welcome!"))

		conv, trigger := h.conversation("Thanks so much!")
		h.run(conv, trigger)

		msgs := h.llm.answer.Sent()[0].Messages
		assert.Empty(t, msgs[len(msgs)-1].ToolCalls)
		assert.NotEqual(t, "tool", string(msgs[len(msgs)-1].Role))
	})

	t.Run("points to a passage already shown instead of sending it again", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.withTokenSavers()
		h.llm.answer.Script(search("pro plan price per seat"), reply("Per seat [1]."))

		conv, trigger := h.conversation("What is the pro plan price per seat?")
		h.run(conv, trigger)

		msgs := h.llm.answer.Sent()[1].Messages
		assert.Contains(t, msgs[len(msgs)-1].Parts[0].Text, "(shown above)")
	})
}
