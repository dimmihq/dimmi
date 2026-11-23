package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

/*
batchFix is a suggestion to drop the Free batch limit from an article,
under a title and a key of its own.
*/
func batchFix(article, title, key, replace string) fake.Step {
	return tools(call("propose_article_edit", map[string]any{
		"article_id": article, "suggestion_title": title, "reason": "Free has no lower limit.", "dedup_key": key,
		"edits": []map[string]any{{
			"find": "- A batch can hold at most 1,000 shipments, or 25 on the Free plan.", "replace": replace,
		}},
	}))
}

/*
TestOperatorRoutineRepeats covers what keeps a routine from repeating
itself: a fix already waiting is not suggested again whatever its key,
and one run reads only its share of articles.
*/
func TestOperatorRoutineRepeats(t *testing.T) {
	t.Parallel()

	t.Run("does not suggest a second time a fix to the same lines under another key", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Batches")
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			batchFix(article, "Fix the batch limit", "stale:batch", "- A batch can hold at most 1,000 shipments."),
			batchFix(article, "Correct the Free limit", "contradiction:free-batch", "- A batch holds up to 1,000."),
			reply("Suggested one fix."),
		)

		run := k.runRoutine(routine, true)

		run.AssertJSONPath("data.changesets_count", float64(1))
		sent := k.llm.Sent()[2].Messages
		assert.Contains(t, sent[len(sent)-1].Parts[0].Text, `the suggestion "Fix the batch limit" already changes`)
	})

	t.Run("does not suggest again what a live run left waiting, and still suggests other lines", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Batches")
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			batchFix(article, "Fix the batch limit", "stale:batch", "- A batch can hold at most 1,000 shipments."),
			reply("Suggested one fix."),
			batchFix(article, "Correct the Free limit", "other:key", "- A batch holds up to 1,000."),
			tools(call("propose_article_edit", map[string]any{
				"article_id": article, "suggestion_title": "Fix the label limit", "reason": "Plans changed.",
				"edits": []map[string]any{{
					"find":    "The Free plan stops at 50 labels a month.",
					"replace": "The Free plan stops at 40 labels a month.",
				}},
			})),
			reply("Suggested one more fix."),
		)

		k.runRoutine(routine, false).AssertJSONPath("data.changesets_count", float64(1))
		k.runRoutine(routine, false).AssertJSONPath("data.changesets_count", float64(1))
	})

	t.Run("lets one run read only its share of articles", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Batches")
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.setRoutine(routine, "max_tool_calls = 40")
		reads := make([]contracts.ToolCall, 21)
		for i := range reads {
			reads[i] = call("get_article_by_id", map[string]any{"article_id": article})
		}
		k.llm.Script(tools(reads...), reply("Read what I could."))

		k.runRoutine(routine, true)

		refused := 0
		for _, m := range k.llm.Sent()[1].Messages {
			for _, part := range m.Parts {
				if strings.Contains(part.Text, "has read 20 articles") {
					refused++
				}
			}
		}
		assert.Equal(t, 1, refused)
	})
}
