package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/operator"
)

func TestOperatorArticleEdits(t *testing.T) {
	t.Parallel()

	t.Run("refuses an edit that would cut most of an article away", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		hc := k.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		long := strings.Repeat("Refunds take five to ten business days to reach the card. ", 20)
		article := k.d.Owner.PostJSON("/api/v1/help-centers/"+hc+"/articles", testkit.JSON{
			"title": "Refunds", "body_markdown": long,
		}).AssertCreated().String("data.id")
		thread := k.thread("")
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Fix refunds", "reason": "Out of date.",
			"operations": []map[string]any{{
				"kind": "article.edit_draft", "article": article, "body": "Say refunds take 3 days instead.",
			}},
		})), reply("Could not."))

		k.ask(thread, "Fix the refunds article").AssertOK().AssertJSONPath("data.changesets.#", float64(0))

		result := k.llm.Sent()[1].Messages[len(k.llm.Sent()[1].Messages)-1].Parts[0].Text
		assert.Contains(t, result, "send edits with the exact passages")
	})
}

func TestOperatorPassageEdits(t *testing.T) {
	t.Parallel()

	t.Run("writes exact passages into a long article", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		hc := k.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		long := strings.Repeat("Labels print in one click. ", 30) + "\n\nRefunds take 14 days.\n"
		article := k.d.Owner.PostJSON("/api/v1/help-centers/"+hc+"/articles", testkit.JSON{
			"title": "Refunds", "body_markdown": long,
		}).AssertCreated().String("data.id")
		thread := k.thread("")
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Fix refund time", "reason": "Refunds now take 5 days.",
			"operations": []map[string]any{{
				"kind": "article.edit_draft", "article": article,
				"edits": []map[string]any{{"find": "Refunds take 14 days.", "replace": "Refunds take 5 days."}},
			}},
		})), reply("Proposed."))

		res := k.ask(thread, "Fix the refund time").AssertOK()
		id := res.String("data.changesets.0.id")
		assert.Contains(t, res.String("data.changesets.0.operations.0.text"), "Refunds take 5 days.")
		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK()
		body := k.d.Owner.GetJSON("/api/v1/articles/" + article).AssertOK().String("data.body_markdown")
		assert.Contains(t, body, "Refunds take 5 days.")
		assert.Contains(t, body, "Labels print in one click.")
	})

	t.Run("refuses a passage the article does not hold", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		hc := k.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		article := k.d.Owner.PostJSON("/api/v1/help-centers/"+hc+"/articles", testkit.JSON{
			"title": "Refunds", "body_markdown": "Refunds take 14 days.",
		}).AssertCreated().String("data.id")
		thread := k.thread("")
		k.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "Fix", "reason": "x",
			"operations": []map[string]any{{
				"kind": "article.edit_draft", "article": article,
				"edits": []map[string]any{{"find": "Refunds take 30 days.", "replace": "y"}},
			}},
		})), reply("Could not."))

		k.ask(thread, "Fix it").AssertOK().AssertJSONPath("data.changesets.#", float64(0))
		result := k.llm.Sent()[1].Messages[len(k.llm.Sent()[1].Messages)-1].Parts[0].Text
		assert.Contains(t, result, "is not in the article")
	})

	t.Run("lets a knowledge template change only articles", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Refund please")
		number := k.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK().String("data.number")
		routine := k.routine(testkit.JSON{"template_key": operator.TemplateStaleness})
		k.llm.Script(tools(call("propose_changes", tagging(t, number, ""))), reply("Tried."))

		k.runRoutine(routine, true).AssertJSONPath("data.changesets_count", float64(0))
		result := k.llm.Sent()[1].Messages[len(k.llm.Sent()[1].Messages)-1].Parts[0].Text
		assert.Contains(t, result, "switched off")
	})
}
