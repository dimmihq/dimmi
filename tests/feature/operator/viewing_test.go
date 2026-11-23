package operator_test

import (
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
	viewing := testkit.JSON{"article_id": article}
	if changeset != "" {
		viewing["changeset_id"] = changeset
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
		note := asked.Parts[len(asked.Parts)-1].Text
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
		note := asked.Parts[len(asked.Parts)-1].Text
		assert.Contains(t, note, `The help article "Batches" is open.`)
		assert.Contains(t, note, "article_id: "+article)
		assert.Contains(t, note, "The article as it reads now:")
		assert.Contains(t, note, "or 25 on the Free plan")
		assert.NotContains(t, note, "A suggestion waits")
	})
}
