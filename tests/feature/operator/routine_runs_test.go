package operator_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
TestRoutineRunSaysWhatItReadAndWhatWaits covers what a run tells a
teammate beyond its outcome: how much it read, and how many of its
suggestions still wait for them.
*/
func TestRoutineRunSaysWhatItReadAndWhatWaits(t *testing.T) {
	t.Parallel()
	k := newDesk(t)
	article := k.article("Refunds")
	routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Check the help center."})
	read := call("get_article_by_id", map[string]any{"article_id": article})
	k.llm.Script(
		tools(read, read),
		tools(call("propose_new_article", map[string]any{
			"article_title": "Returns", "article_body": "Send it back within 30 days.",
			"suggestion_title": "Add a returns article", "reason": "Nothing covers returns.",
		})),
		reply("I suggested one article."),
	)

	run := k.runRoutine(routine, false)

	run.AssertJSONPath("data.read.articles", float64(1)).
		AssertJSONPath("data.read.conversations", float64(0)).
		AssertJSONPath("data.suggestions_waiting", float64(1))
	list := k.d.Owner.GetJSON("/api/v1/operator/routines/" + routine + "/runs").AssertOK()
	list.AssertJSONPath("data.0.suggestions_waiting", float64(1)).AssertJSONPath("data.0.read.articles", float64(1))

	suggestion := k.d.Owner.GetJSON("/api/v1/operator/changesets").AssertOK().String("data.0.id")
	k.d.Owner.PostJSON("/api/v1/operator/changesets/"+suggestion+"/declines", testkit.JSON{}).AssertOK()

	k.d.Owner.GetJSON("/api/v1/operator/routines/"+routine+"/runs").AssertOK().
		AssertJSONPath("data.0.suggestions_waiting", float64(0))
}
