package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
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
		assert.Contains(t, sent[0].System[0].Text, "find_related_conversations")
		found := sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text
		assert.Contains(t, found, "handled by nobody")
		assert.Contains(t, found, "Of these 1 conversations:")
		assert.Contains(t, found, "Assigned to: none")
	})

	t.Run("finds the conversations most like the open one with no topic, and hands them up front", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		open := k.conversation("My export is stuck again.")
		solved := k.conversation("The export is stuck since Monday.")
		k.d.Owner.PatchJSON("/api/v1/conversations/"+solved, testkit.JSON{"status": "closed"}).AssertOK()
		k.d.TC.Drain()
		thread := k.thread(open)
		k.llm.Script(tools(call("find_related_conversations", map[string]any{})), reply("Assign to Support."))

		k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{
			"body": "Who should take this?", "viewing": testkit.JSON{"page": "conversation", "id": open},
		}).AssertOK()

		sent := k.llm.Sent()
		found := sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text
		assert.Contains(t, found, "what was asked and how it was answered:")
		assert.Contains(t, found, "The export is stuck since Monday.")
		assert.Contains(t, found, "Of these 1 conversations:")
		var given []string
		for _, part := range sent[0].Messages[len(sent[0].Messages)-1].Parts {
			given = append(given, part.Text)
		}
		assert.Contains(t, strings.Join(given, "\n"), "The conversations most like this one")
	})

	t.Run("says to call it a guess when nothing is related", func(t *testing.T) {
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
		assert.Contains(t, back(2), "say your recommendation is a guess")
	})
}
