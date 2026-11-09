package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestOperatorContentChanges(t *testing.T) {
	t.Parallel()

	t.Run("assigns, closes, renames and notes a conversation, and undoes it all", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Charged twice.")
		team := k.d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		thread := k.thread(conv)
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Hand to Billing", "reason": "It is a billing question.",
			"operations": []map[string]any{
				{"kind": "conversation.assign", "team": team},
				{"kind": "conversation.title", "title": "Double charge in March"},
				{"kind": "conversation.note", "body": "Refund the second charge."},
				{"kind": "conversation.status", "status": "closed"},
				{"kind": "contact.update", "name": "Ana M. Lima"},
			},
		})), reply("Proposed."))
		id := k.ask(thread, "Hand this over").AssertOK().String("data.changesets.0.id")

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.status", "applied")
		shown := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK()
		assert.Equal(t, "Billing", shown.String("data.team.name"))
		assert.Equal(t, "Double charge in March", shown.String("data.subject"))
		assert.Equal(t, "closed", shown.String("data.status"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM messages WHERE type = 'note' AND deleted_at IS NULL"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM contacts WHERE name = 'Ana M. Lima'"))

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/reversions", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.operations.0.outcome", "reverted")
		after := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK()
		assert.Empty(t, after.String("data.team.name"))
		assert.Equal(t, "Refund", after.String("data.subject"))
		assert.Equal(t, "open", after.String("data.status"))
		assert.Equal(t, 0, k.count("SELECT count(*) FROM messages WHERE type = 'note' AND deleted_at IS NULL"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM contacts WHERE name = 'Ana Lima'"))
	})

	t.Run("drafts and edits articles and procedures without publishing, and undoes them", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		hc := k.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		article := k.d.Owner.PostJSON("/api/v1/help-centers/"+hc+"/articles", testkit.JSON{
			"title": "Refunds", "body_markdown": "Old text.",
		}).AssertCreated().String("data.id")
		procedure := k.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": "Refunds"}).AssertCreated().
			String("data.id")
		thread := k.thread("")
		definition := map[string]any{
			"description": "Refund requests.",
			"steps":       []map[string]any{{"id": "s1", "kind": "instruction", "text": "Check the charge."}},
			"handover":    []string{},
		}
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Fix refunds docs", "reason": "Outdated.",
			"operations": []map[string]any{
				{"kind": "article.create_draft", "title": "Double charges", "body": "If you were charged twice..."},
				{"kind": "article.edit_draft", "article": article, "body": "New text."},
				{"kind": "procedure.edit_draft", "procedure": procedure, "definition": definition},
			},
		})), reply("Proposed."))
		res := k.ask(thread, "Fix the refund docs").AssertOK()
		id := res.String("data.changesets.0.id")
		assert.Equal(t, "Draft a new article “Double charges”", res.String("data.changesets.0.operations.0.label"))

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK()
		assert.Equal(t, 1, k.count("SELECT count(*) FROM articles WHERE title = 'Double charges' AND status = 'draft'"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM articles WHERE body_markdown = 'New text.'"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM procedures WHERE draft->>'description' = 'Refund requests.'"))

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/reversions", testkit.JSON{}).AssertOK()
		assert.Equal(t, 0,
			k.count("SELECT count(*) FROM articles WHERE title = 'Double charges' AND deleted_at IS NULL"))
		assert.Equal(t, 1, k.count("SELECT count(*) FROM articles WHERE body_markdown = 'Old text.'"))
		assert.Equal(t, 0, k.count("SELECT count(*) FROM procedures WHERE draft->>'description' = 'Refund requests.'"))
	})

	t.Run("keeps a field a teammate changed after Operator, and records a failed apply", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Charged twice.")
		thread := k.thread(conv)
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Set priority", "reason": "Urgent.",
			"operations": []map[string]any{{"kind": "conversation.priority", "priority": "urgent"}},
		})), reply("Proposed."))
		id := k.ask(thread, "Prioritize").AssertOK().String("data.changesets.0.id")
		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK()
		k.d.Owner.PatchJSON("/api/v1/conversations/"+conv, testkit.JSON{"priority": "low"}).AssertOK()

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/reversions", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.operations.0.outcome", "skipped")
		k.d.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.priority", "low")

		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Assign", "reason": "Owner.",
			"operations": []map[string]any{{"kind": "conversation.tag", "tags": []string{"vip"}}},
		})), reply("Proposed."))
		failing := k.ask(thread, "Tag VIP").AssertOK().String("data.changesets.0.id")
		k.d.TC.FailWrites("conversation_tag", "INSERT")
		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+failing+"/applications", testkit.JSON{}).AssertStatus(500)
		assert.Equal(t, 1, k.count("SELECT count(*) FROM operator_changesets WHERE status = 'failed'"))
	})
}
