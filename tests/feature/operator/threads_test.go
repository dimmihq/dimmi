package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestOperatorConversationThreads(t *testing.T) {
	t.Parallel()

	t.Run("keeps several threads about one conversation and opens the latest", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("My export is stuck.")
		first := k.thread(conv)
		k.llm.Script(reply("First."), reply("Second."))
		k.ask(first, "What happened?").AssertOK()

		second := k.d.Owner.PostJSON("/api/v1/operator/threads", testkit.JSON{"conversation_id": conv, "fresh": true}).
			AssertOK().String("data.id")
		k.ask(second, "Anything else?").AssertOK()

		assert.NotEqual(t, first, second)
		assert.Equal(t, second, k.thread(conv))
		listed := k.d.Owner.GetJSON("/api/v1/operator/threads?filter[conversation]=" + conv).AssertOK()
		assert.Equal(t, second, listed.String("data.0.id"))
		assert.Equal(t, first, listed.String("data.1.id"))
		assert.Empty(t, listed.String("data.2.id"))
	})

	t.Run("hides a conversation's threads from a teammate who cannot see it", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("My export is stuck.")
		other, _ := k.d.Join(t, enums.RoleAdmin)

		listed := other.GetJSON("/api/v1/operator/threads?filter[conversation]=" + conv).AssertOK()

		assert.Empty(t, listed.String("data.0.id"))
	})

	t.Run("keeps a brief on the conversation as a note when asked", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("My export is stuck.")
		k.llm.Script(reply("Ana's export is stuck at 40%."), reply("Still stuck."))

		k.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/operator/brief", testkit.JSON{"attach": true}).AssertOK()
		k.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/operator/brief", testkit.JSON{}).AssertOK()

		assert.Equal(t, 1, k.count("SELECT count(*) FROM messages WHERE type = 'note' "+
			"AND body LIKE '**Brief from Operator**%Ana''s export is stuck at 40%'"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM messages WHERE type = 'note'"))
	})

	t.Run("briefs on the thread named", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("My export is stuck.")
		first := k.thread(conv)
		k.d.Owner.PostJSON("/api/v1/operator/threads", testkit.JSON{"conversation_id": conv, "fresh": true}).AssertOK()
		k.llm.Script(reply("Brief."))

		res := k.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/operator/brief",
			testkit.JSON{"thread_id": first}).AssertOK()

		assert.Equal(t, first, res.String("data.id"))
	})
}
