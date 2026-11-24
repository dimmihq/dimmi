package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
TestAiTeamNote lets the agent tell the team something without giving
the conversation away.
*/
func TestAiTeamNote(t *testing.T) {
	t.Parallel()

	t.Run("leaves an internal note and keeps the conversation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(
			tools(call("leave_note", `{"note":"Buyer chose another tool because it has a waitlist [1]. Polite."}`)),
			reply("Thanks for letting me know. I've noted it."))

		conv, trigger := h.conversation("Hi, we decided to go with another tool in the end.")
		res := h.run(conv, trigger)
		require.Equal(t, "greeted", res.Outcome, "a note is not a handover")

		s := h.load(conv, res.RunID)
		assert.NotEqual(t, "handed_off", s.conv.AiState, "the agent keeps the conversation")
		assert.Nil(t, s.conv.AiHandoffReason)
		require.Len(t, s.notes(), 1)
		assert.Equal(t, "Buyer chose another tool because it has a waitlist. Polite.", str(s.notes()[0].Body),
			"citation marks are for the customer's reply, not the team's note")
		assert.Equal(t, "agent", s.notes()[0].AuthorType, "the thread shows the agent left it")
		require.Len(t, s.agentReplies(), 1)
		assert.Equal(t, "Thanks for letting me know. I've noted it.", str(s.agentReplies()[0].Body))

		told := lastText(h.llm.answer.Sent()[1])
		assert.Contains(t, told, "the conversation stays with you")
		assert.Contains(t, told, "Nobody was asked to reply",
			"so the agent does not tell the customer someone will be in touch")
	})

	t.Run("refuses an empty note, and more notes than a reply may leave", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(
			tools(call("leave_note", `{"note":"  "}`)),
			tools(call("leave_note", `{"note":"First."}`), call("leave_note", `{"note":"Second."}`),
				call("leave_note", `{"note":"Third."}`)),
			reply("I've noted it."))

		conv, trigger := h.conversation("Hi, we decided to go with another tool in the end.")
		res := h.run(conv, trigger)

		sent := h.llm.answer.Sent()
		empty := sent[1].Messages[len(sent[1].Messages)-1]
		assert.True(t, empty.ToolError)
		assert.Contains(t, lastText(sent[2]), `"ok":false`)
		s := h.load(conv, res.RunID)
		require.Len(t, s.notes(), 2)
		assert.Equal(t, "First.", str(s.notes()[0].Body))
		assert.Equal(t, "Second.", str(s.notes()[1].Body))
	})
}
