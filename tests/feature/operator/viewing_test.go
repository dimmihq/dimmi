package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
askViewing asks Operator on a thread while an article and one of its
suggestions are open.
*/
func (k *desk) askViewing(thread, text, article, changeset string) *testkit.Response {
	k.t.Helper()
	viewing := testkit.JSON{"page": "article", "id": article}
	if changeset != "" {
		viewing["suggestion_id"] = changeset
	}
	return k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{
		"body": text, "viewing": viewing,
	})
}

/*
TestOperatorViewing covers a teammate asking Operator from a page: what
Operator is told is open, and where a change it makes for them lands.
*/
func TestOperatorViewing(t *testing.T) {
	t.Parallel()

	t.Run("tells the model what is open and adds its change to the open suggestion in place", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Batches")
		thread := k.thread("")
		k.llm.Script(
			batchFix(article, "Fix the batch limit", "stale:batch", "- A batch can hold at most 1,000 shipments."),
			reply("Suggested."),
		)
		suggestion := k.ask(thread, "Fix the batch article").AssertOK().String("data.changesets.0.id")
		k.llm.Script(
			tools(call("get_suggestion_by_id", map[string]any{"suggestion_id": suggestion[:len(suggestion)-1]})),
			tools(call("propose_article_edits", map[string]any{"fixes": []map[string]any{{
				"article_id": article, "suggestion_title": "Say 500", "reason": "The teammate asked for 500.",
				"edits": []map[string]any{{
					"find":    "- A batch can hold at most 1,000 shipments.",
					"replace": "- A batch can hold at most 500 shipments.",
				}},
			}}})),
			reply("Changed it to 500."),
		)

		second := k.askViewing(thread, "Make it 500 instead", article, suggestion).AssertOK()

		second.AssertJSONPath("data.changesets.#", float64(0))
		sent := k.llm.Sent()
		asked := sent[2].Messages[len(sent[2].Messages)-1]
		note := asked.Parts[len(asked.Parts)-2].Text
		assert.Contains(t, asked.Parts[len(asked.Parts)-1].Text, "Write your answer for a busy teammate")
		assert.Contains(t, note, `The help article "Batches" is open.`)
		assert.Contains(t, note, "article_id: "+article)
		assert.Contains(t, note, "suggestion_id: "+suggestion)
		assert.Contains(t, note, `Made in a chat with Operator`)
		assert.Contains(t, note, "Reason: Free has no lower limit.")
		assert.Contains(t, note, "Lines it removes:")
		assert.Contains(t, note, "The article as suggested")
		assert.Contains(t, note, "- A batch can hold at most 1,000 shipments.")
		read := sent[3].Messages[len(sent[3].Messages)-1].Parts[0].Text
		assert.Contains(t, read, "Lines it removes:")
		assert.Contains(t, read, "The article as suggested")
		added := sent[4].Messages[len(sent[4].Messages)-1].Parts[0].Text
		assert.Contains(t, added, `Added to the suggestion "Fix the batch limit"`)
		list := k.d.Owner.GetJSON("/api/v1/operator/changesets?filter[status]=pending").AssertOK()
		list.AssertJSONPath("data.#", float64(1))
		assert.Contains(t, list.String("data.0.operations.0.text"), "at most 500 shipments")
		assert.Contains(t, list.String("data.0.reason"), "The teammate asked for 500.")
	})

	t.Run("makes a new suggestion when none is open on the article", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Batches")
		k.llm.Script(
			batchFix(article, "Fix the batch limit", "stale:batch", "- A batch can hold at most 1,000 shipments."),
			reply("Suggested."),
		)

		res := k.askViewing(k.thread(""), "Drop the Free limit", article, "").AssertOK()

		res.AssertJSONPath("data.changesets.#", float64(1))
		asked := k.llm.Sent()[0].Messages[len(k.llm.Sent()[0].Messages)-1]
		note := asked.Parts[len(asked.Parts)-2].Text
		assert.Contains(t, note, `The help article "Batches" is open.`)
		assert.Contains(t, note, "article_id: "+article)
		assert.Contains(t, note, "The article as it reads now:")
		assert.Contains(t, note, "or 25 on the Free plan")
		assert.NotContains(t, note, "A suggestion waits")
	})

	t.Run("tells the model about an open conversation and help center, each from its source", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("My export is stuck.")
		hc := k.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		thread := k.thread("")
		k.llm.Script(reply("It is about an export."), reply("That is your help center."), reply("Nothing open."))

		k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{
			"body": "What is this about?", "viewing": testkit.JSON{"page": "conversation", "id": conv},
		}).AssertOK()
		k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{
			"body": "What is this?", "viewing": testkit.JSON{"page": "help_center", "id": hc},
		}).AssertOK()
		k.d.Owner.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{
			"body": "And this?", "viewing": testkit.JSON{"page": "spaceship", "id": hc},
		}).AssertOK()

		sent := k.llm.Sent()
		last := func(i int) string {
			var all []string
			for _, part := range sent[i].Messages[len(sent[i].Messages)-1].Parts {
				all = append(all, part.Text)
			}
			return strings.Join(all, "\n")
		}
		assert.Contains(t, last(0), "Conversation #")
		assert.Contains(t, last(0), "My export is stuck.")
		assert.Contains(t, last(0), "How this workspace replies to customers.")
		assert.Contains(t, last(0), "How to draft a reply:")
		assert.Contains(t, last(1), "A help center is open.")
		assert.Contains(t, last(1), "help_center_id: "+hc)
		assert.Contains(t, last(1), "Public link, what customers see: http")
		assert.NotContains(t, last(2), "has open right now", "a kind of page Operator does not know adds nothing")
	})
}
