package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
TestOperatorRelatedConversations covers finding what the team did with
conversations like the one open, so a recommendation can rest on it.
*/
func TestOperatorRelatedConversations(t *testing.T) {
	t.Parallel()

	t.Run("lists similar conversations with who handled them, leaving out the one open", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		open := k.conversation("My export is stuck again.")
		k.conversation("The export is stuck since Monday.")
		thread := k.thread(open)
		k.llm.Script(
			tools(call("find_related_conversations", map[string]any{"topic": "export stuck"})),
			reply("Assign to Support."),
		)

		k.ask(thread, "Who should take this?").AssertOK()

		sent := k.llm.Sent()
		assert.Contains(t, sent[0].System[0].Text, "call\n  find_related_conversations")
		found := sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text
		assert.Contains(t, found, "handled by nobody")
		assert.Contains(t, found, "Of these 1 conversations:")
		assert.Contains(t, found, "Assigned to: none")
	})

	t.Run("says to call it a guess when nothing is related, and asks for a missing topic", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		thread := k.thread(k.conversation("My export is stuck."))
		k.llm.Script(
			tools(call("find_related_conversations", map[string]any{"topic": "zeppelin moorings"})),
			tools(call("find_related_conversations", map[string]any{})),
			reply("No history."),
		)

		k.ask(thread, "Who should take this?").AssertOK()

		sent := k.llm.Sent()
		back := func(i int) string { return sent[i].Messages[len(sent[i].Messages)-1].Parts[0].Text }
		assert.Contains(t, back(1), "say your recommendation is a guess")
		assert.Contains(t, back(2), `"topic" is missing`)
	})
}
