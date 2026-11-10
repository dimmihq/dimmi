package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestOperatorTools(t *testing.T) {
	t.Parallel()

	t.Run("reads tags, teammates, customers and conversations through the asking teammate", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Where is my order?")
		thread := k.thread(conv)
		k.llm.Script(
			tools(call("list_tags", map[string]any{}), call("list_teammates", map[string]any{}),
				call("get_customer", map[string]any{}), call("search_conversations", map[string]any{"query": "order"})),
			reply("Done."),
		)

		res := k.ask(thread, "Look around").AssertOK()

		for i := range 4 {
			assert.Equal(t, "true", res.String("data.message.steps."+itoa(i)+".ok"), res.String("data.message.steps"))
		}
		results := k.llm.Sent()[1].Messages
		assert.Contains(t, results[len(results)-2].Parts[0].Text, "Customer <untrusted>Ana Lima</untrusted>")
		assert.Contains(t, results[len(results)-1].Parts[0].Text, "#1")
	})

	t.Run("lists what is open, what is unassigned and what the agent handed over", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.conversation("Where is my order?")
		k.conversation("My card was charged twice")
		thread := k.thread("")
		k.llm.Script(
			tools(call("list_conversations", map[string]any{}),
				call("list_conversations", map[string]any{"assignee": "unassigned", "status": "open"}),
				call("list_conversations", map[string]any{"ai": "handed_off"}),
				call("list_conversations", map[string]any{"waiting": true})),
			reply("Two are open."),
		)

		res := k.ask(thread, "What is open right now?").AssertOK()

		assert.Equal(t, "Listed 2 open conversations", res.String("data.message.steps.0.summary"))
		assert.Equal(t, "Listed 2 open, unassigned conversations", res.String("data.message.steps.1.summary"))
		assert.Equal(t, "Listed 0 open, handed_off conversations", res.String("data.message.steps.2.summary"))
		assert.Equal(t, "Listed 2 open, waiting conversations", res.String("data.message.steps.3.summary"))
		results := k.llm.Sent()[1].Messages
		assert.Contains(t, results[len(results)-4].Parts[0].Text, "#1")
		assert.Contains(t, results[len(results)-4].Parts[0].Text, "#2")
		assert.Contains(t, results[len(results)-4].Parts[0].Text, "last from the customer")
		assert.Contains(t, results[len(results)-2].Parts[0].Text, "No conversations match.")
	})

	t.Run("reads knowledge, articles, procedures and the agent's runs", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Help")
		hc := k.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		article := k.d.Owner.PostJSON("/api/v1/help-centers/"+hc+"/articles", testkit.JSON{
			"title": "Refunds", "body_markdown": "We refund within 30 days.",
		}).AssertCreated().String("data.id")
		procedure := k.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": "Refunds"}).AssertCreated().
			String("data.id")
		thread := k.thread(conv)
		k.llm.Script(
			tools(call("search_knowledge", map[string]any{"query": "refund"}), call("list_articles", map[string]any{}),
				call("get_article", map[string]any{"article": article}), call("list_procedures", map[string]any{}),
				call("get_procedure", map[string]any{"procedure": procedure}), call("agent_runs", map[string]any{})),
			reply("Done."),
		)

		res := k.ask(thread, "Check the knowledge").AssertOK()

		for i := range 6 {
			assert.Equal(t, "true", res.String("data.message.steps."+itoa(i)+".ok"), res.String("data.message.steps"))
		}
		results := k.llm.Sent()[1].Messages
		assert.Contains(t, results[len(results)-4].Parts[0].Text, "We refund within 30 days.")
		assert.Contains(t, results[len(results)-1].Parts[0].Text, "has not run")
	})

	t.Run("says plainly when a tool is refused, unknown or given nonsense", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		thread := k.thread("")
		k.llm.Script(
			tools(call("get_conversation", map[string]any{}), call("make_coffee", map[string]any{}),
				call("get_article", map[string]any{"article": "nope"}),
				call("workspace_report", map[string]any{"since": "yesterday"}),
				call("draft_reply", map[string]any{"body": "Hi"})),
			reply("Some of that failed."),
		)

		res := k.ask(thread, "Do things").AssertOK()

		for i := range 5 {
			assert.Equal(t, "false", res.String("data.message.steps."+itoa(i)+".ok"))
		}
		assert.Contains(t, res.String("data.message.steps.3.summary"), "since must be YYYY-MM-DD")
	})

	t.Run("keeps reports from teammates who cannot see every conversation", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		member, row := k.d.Join(t, enums.RoleMember)
		_, err := k.d.TC.Operator().Exec(t.Context(),
			"UPDATE workspace_members SET conversation_access = 'assigned' WHERE id = $1", row.ID)
		require.NoError(t, err)
		thread := member.PostJSON("/api/v1/operator/threads", testkit.JSON{}).AssertOK().String("data.id")
		k.llm.Script(tools(call("workspace_report", map[string]any{})), reply("I cannot."))

		res := member.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{"body": "Report"}).
			AssertOK()

		assert.Equal(t, "false", res.String("data.message.steps.0.ok"))
		member.PatchJSON("/api/v1/operator/settings", testkit.JSON{"enabled": false}).AssertForbidden()
	})

	t.Run("withholds tools whose changes are switched off", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		thread := k.thread("")
		groups := testkit.JSON{}
		for _, g := range []string{"conversations", "notes", "contacts", "articles", "procedures"} {
			groups[g] = "off"
		}
		k.d.Owner.PatchJSON("/api/v1/operator/settings", testkit.JSON{"policies": groups}).AssertOK()
		k.llm.Script(reply("Hello."))

		k.ask(thread, "Hi").AssertOK()

		for _, spec := range k.llm.Sent()[0].Tools {
			assert.NotEqual(t, "propose_changes", spec.Name)
		}
	})

	t.Run("lists, shows and deletes the teammate's threads", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		thread := k.thread("")
		k.d.Owner.GetJSON("/api/v1/operator/threads?filter[scope]=workspace").AssertOK().AssertJSONMissing("data.0")
		k.llm.Script(reply("Hi."))
		k.ask(thread, "Hello there").AssertOK()

		k.d.Owner.GetJSON("/api/v1/operator/threads?filter[scope]=workspace").AssertOK().
			AssertJSONPath("data.0.title", "Hello there")
		k.d.Owner.Delete("/api/v1/operator/threads/" + thread).AssertNoContent()
		k.d.Owner.GetJSON("/api/v1/operator/threads/" + thread).AssertNotFound()
		k.d.Owner.Delete("/api/v1/operator/threads/" + thread).AssertNotFound()
		k.d.Owner.Delete("/api/v1/operator/memories/opy_nope").AssertNotFound()
	})

	t.Run("stores the answer even when the model fails", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		thread := k.thread("")
		k.llm.Script(fakeFailure())

		res := k.ask(thread, "Hello").AssertOK()

		assert.Equal(t, "failed", res.String("data.message.status"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM ai_runs WHERE kind = 'task' AND status = 'failed'"))
	})
}
