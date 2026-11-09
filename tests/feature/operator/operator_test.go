package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestOperatorThreads(t *testing.T) {
	t.Parallel()

	t.Run("answers on a conversation's thread after reading it, and records a task run", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("I was charged twice for March.")
		thread := k.thread(conv)
		k.llm.Script(tools(call("get_conversation", map[string]any{})), reply("Ana was charged twice; refund one."))

		res := k.ask(thread, "What does Ana want?").AssertOK()

		assert.Equal(t, "Ana was charged twice; refund one.", res.String("data.message.body"))
		assert.Equal(t, "What does Ana want?", res.String("data.asked.body"))
		assert.Equal(t, "get_conversation", res.String("data.message.steps.0.tool"))
		assert.Equal(t, "true", res.String("data.message.steps.0.ok"))
		sent := k.llm.Sent()
		assert.Contains(t, sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text,
			"<untrusted>I was charged twice for March.</untrusted>")
		assert.Equal(t, 1, k.count("SELECT count(*) FROM ai_runs WHERE kind = 'task' AND status = 'completed'"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM ai_spend_periods WHERE cost_micros > 0 AND runs_count = 0"))

		shown := k.d.Owner.GetJSON("/api/v1/operator/threads/" + thread).AssertOK()
		assert.Equal(t, "What does Ana want?", shown.String("data.title"))
		assert.Equal(t, "assistant", shown.String("data.messages.1.role"))
	})

	t.Run("opens the same thread for a conversation twice and keeps it private", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Hello")
		first, second := k.thread(conv), k.thread(conv)
		assert.Equal(t, first, second)

		other, _ := k.d.Join(t, enums.RoleAdmin)
		other.GetJSON("/api/v1/operator/threads/" + first).AssertNotFound()
	})

	t.Run("briefs a conversation without showing the question", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("My export is stuck.")
		k.llm.Script(tools(call("get_conversation", map[string]any{})), reply("Ana's export is stuck."))

		res := k.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/operator/brief", testkit.JSON{}).AssertOK()

		assert.Equal(t, "assistant", res.String("data.messages.0.role"))
		assert.Equal(t, "Ana's export is stuck.", res.String("data.messages.0.body"))
		assert.Empty(t, res.String("data.messages.1.id"))
	})

	t.Run("hands a reply draft to the teammate without sending it", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Where is my invoice?")
		thread := k.thread(conv)
		k.llm.Script(tools(call("draft_reply", map[string]any{"body": "Hi Ana, here it is."})), reply("Drafted."))

		res := k.ask(thread, "Draft a reply").AssertOK()

		assert.Equal(t, "Hi Ana, here it is.", res.String("data.message.steps.0.draft"))
		assert.Equal(t, 0, k.count("SELECT count(*) FROM messages WHERE author_type = 'member'"))
	})

	t.Run("remembers what the teammate asks and forgets it on request", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		thread := k.thread("")
		k.llm.Script(tools(call("remember", map[string]any{"fact": "Answer in bullet points."})), reply("Noted."))
		k.ask(thread, "Always answer in bullet points").AssertOK()

		listed := k.d.Owner.GetJSON("/api/v1/operator/memories").AssertOK()
		assert.Equal(t, "Answer in bullet points.", listed.String("data.0.body"))

		k.llm.Script(reply("Sure."))
		k.ask(thread, "Hi").AssertOK()
		assert.Contains(t, k.llm.Sent()[2].System[0].Text, "Answer in bullet points.")

		k.d.Owner.Delete("/api/v1/operator/memories/" + listed.String("data.0.id")).AssertNoContent()
		assert.Equal(t, 0, k.count("SELECT count(*) FROM operator_memories"))
	})

	t.Run("reports on the workspace for teammates who see every conversation", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.conversation("One")
		thread := k.thread("")
		k.llm.Script(tools(call("workspace_report", map[string]any{})), reply("One conversation this month."))

		res := k.ask(thread, "How busy were we?").AssertOK()

		assert.Equal(t, "true", res.String("data.message.steps.0.ok"))
		assert.Contains(t, k.llm.Sent()[1].Messages[2].Parts[0].Text, "Conversations opened: 1")
	})

	t.Run("refuses to answer while switched off or without a model", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		thread := k.thread("")
		k.d.Owner.PatchJSON("/api/v1/operator/settings", testkit.JSON{"enabled": false}).AssertOK()

		k.ask(thread, "Hello").AssertStatus(409).AssertJSONPath("code", "operator_disabled")
		assert.Empty(t, k.llm.Sent())
	})
}
