package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

const allowance = "## What it means\n\nYour plan's monthly label allowance is used up and the plan does not " +
	"allow overage. The Free plan stops at 50 labels a month.\n\n## Why it happens\n\n" +
	"- A batch can hold at most 1,000 shipments, or 25 on the Free plan.\n\n## Limits\n\n" +
	"A batch can hold at most 1,000 shipments, or 25 on the Free plan.\n"

/*
article makes a help article about allowances and returns its id.
*/
func (k *desk) article(title string) string {
	k.t.Helper()
	hc := k.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
	return k.d.Owner.PostJSON("/api/v1/help-centers/"+hc+"/articles", testkit.JSON{
		"title": title, "body_markdown": allowance,
	}).AssertCreated().String("data.id")
}

/*
toolResult is what the last tool call of the first model call returned.
*/
func (k *desk) toolResult() string {
	k.t.Helper()
	sent := k.llm.Sent()[1].Messages
	return sent[len(sent)-1].Parts[0].Text
}

func TestOperatorArticleTools(t *testing.T) {
	t.Parallel()

	t.Run("offers article fixes through a tool that requires the article and its edits", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.llm.Script(reply("Hello."))

		k.ask(k.thread(""), "Hello").AssertOK()

		var found bool
		for _, spec := range k.llm.Sent()[0].Tools {
			schema := string(spec.Schema)
			if spec.Name == "propose_article_edit" {
				found = true
				assert.Contains(t, schema, `"required":["article_id","edits","suggestion_title","reason"]`)
			}
			if spec.Name == "propose_changes" {
				assert.NotContains(t, schema, "article.edit_draft")
			}
		}
		assert.True(t, found)
	})

	t.Run("fixes a passage quoted with slightly different words", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Label limit")
		k.llm.Script(tools(call("propose_article_edit", map[string]any{
			"article_id": article, "suggestion_title": "Fix the label allowance", "reason": "Plans changed.",
			"edits": []map[string]any{{
				"find": "Your plan's monthly label allowance is used up and your plan does not allow overage. " +
					"The Free plan stops at 50 labels a month.",
				"replace": "Your monthly labels are used up. The Free plan stops at 50 labels a month.",
			}},
		})), reply("Suggested."))

		res := k.ask(k.thread(""), "Fix the label limit article").AssertOK()

		assert.Contains(t, res.String("data.changesets.0.operations.0.text"), "Your monthly labels are used up.")
		assert.NotContains(t, res.String("data.changesets.0.operations.0.text"), "does not allow overage")
		assert.Contains(t, k.toolResult(), "worded a little differently")
	})

	t.Run("replaces a passage everywhere the article repeats it", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Batches")
		k.llm.Script(tools(call("propose_article_edit", map[string]any{
			"article_id": article, "suggestion_title": "Fix the batch limit", "reason": "Free has no lower limit.",
			"edits": []map[string]any{
				{
					"find":    "A batch can hold at most 1,000 shipments, or 25 on the Free plan.",
					"replace": "A batch can hold at most 1,000 shipments.",
				},
				{"find": "## Limits", "replace": "## Limits"},
			},
		})), reply("Suggested."))

		res := k.ask(k.thread(""), "Fix the batch article").AssertOK()

		text := res.String("data.changesets.0.operations.0.text")
		assert.Equal(t, 0, strings.Count(text, "25 on the Free plan"))
		assert.Equal(t, 2, strings.Count(text, "A batch can hold at most 1,000 shipments."))
		assert.Contains(t, k.toolResult(), "all 2 places")
	})

	t.Run("names the closest passage when the numbers of a quote are wrong", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Label limit")
		k.llm.Script(tools(call("propose_article_edit", map[string]any{
			"article_id": article, "title": "Fix", "reason": "x",
			"edits": []map[string]any{{"find": "The Free plan stops at 80 labels a month.", "replace": "y"}},
		})), reply("Could not."))

		k.ask(k.thread(""), "Fix it").AssertOK().AssertJSONPath("data.changesets.#", float64(0))

		assert.Contains(t, k.toolResult(), `The closest passage is "The Free plan stops at 50 labels a month."`)
	})

	t.Run("reads edits a model sent as a string", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Label limit")
		k.llm.Script(tools(call("propose_article_edit", map[string]any{
			"article_id": article, "title": "Fix", "reason": "x",
			"edits": `[{"find":"The Free plan stops at 50 labels a month.","replace":"Free stops at 50."}]`,
		})), reply("Suggested."))

		res := k.ask(k.thread(""), "Fix it").AssertOK()

		assert.Contains(t, res.String("data.changesets.0.operations.0.text"), "Free stops at 50.")
	})

	t.Run("says which field is missing", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.article("Label limit")
		k.llm.Script(
			tools(call("propose_article_edit", map[string]any{"title": "Fix", "reason": "x"})),
			tools(call("propose_article_edit", map[string]any{
				"article_id": "Label limit", "title": "Fix", "reason": "x",
				"edits": []map[string]any{{"find": "a", "replace": "b"}},
			})),
			reply("Could not."),
		)

		k.ask(k.thread(""), "Fix it").AssertOK().AssertJSONPath("data.changesets.#", float64(0))

		assert.Contains(t, k.toolResult(), `"article_id" is missing`)
		sent := k.llm.Sent()[2].Messages
		assert.Contains(t, sent[len(sent)-1].Parts[0].Text, "never the title")
	})

	t.Run("drafts a new article", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.llm.Script(tools(call("propose_new_article", map[string]any{
			"article_title": "Set up SSO", "article_body": "Turn on SSO under Settings.",
			"suggestion_title": "Add a guide to SSO setup", "reason": "Three customers asked.",
		})), reply("Suggested."))

		res := k.ask(k.thread(""), "Write an SSO guide").AssertOK()

		assert.Equal(t, "Add a guide to SSO setup", res.String("data.changesets.0.title"))
		assert.Equal(t, "article.create_draft", res.String("data.changesets.0.operations.0.kind"))
	})
}

func TestOperatorToolNames(t *testing.T) {
	t.Parallel()

	t.Run("names every argument that carries an id or a number for what it is", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.llm.Script(reply("Hello."))

		k.ask(k.thread(""), "Hello").AssertOK()

		schemas := map[string]string{}
		for _, spec := range k.llm.Sent()[0].Tools {
			schemas[spec.Name] = string(spec.Schema)
		}
		assert.Contains(t, schemas["get_article_by_id"], `"required":["article_id"]`)
		assert.Contains(t, schemas["get_procedure_by_id"], `"required":["procedure_id"]`)
		assert.Contains(t, schemas["get_conversation_by_number"], `"conversation_number"`)
		assert.Contains(t, schemas["get_customer_by_conversation_number"], `"conversation_number"`)
		assert.Contains(t, schemas["propose_changes"], `"required":["suggestion_title","reason","operations"]`)
		assert.NotContains(t, schemas, "get_article")
	})

	t.Run("still runs a tool called by the name and arguments it had before", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Label limit")
		k.llm.Script(tools(call("get_article", map[string]any{"article": article})), reply("Read."))

		k.ask(k.thread(""), "Read the label article").AssertOK()

		assert.Contains(t, k.toolResult(), "The Free plan stops at 50 labels a month.")
	})
}
