package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
split is a main agent's call that hands parts of the work to subagents.
*/
func split(titles ...string) contracts.ToolCall {
	parts := make([]map[string]any, len(titles))
	for i, title := range titles {
		parts[i] = map[string]any{"title": title, "task": "Audit the help articles at offset 0, limit 25."}
	}
	return call("run_subagents", map[string]any{"subagents": parts})
}

/*
toolNames lists the tools one model call was offered.
*/
func toolNames(req contracts.ChatRequest) []string {
	names := make([]string, len(req.Tools))
	for i, spec := range req.Tools {
		names[i] = spec.Name
	}
	return names
}

/*
TestOperatorRoutineSubagents covers a routine splitting its work: what
a subagent is given, what it may do, and how its report, steps and
report come back to the run.
*/
func TestOperatorRoutineSubagents(t *testing.T) {
	t.Parallel()

	t.Run("runs a subagent that reports back, and leaves the suggesting to the main agent", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		article := k.article("Batches")
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			tools(split("Articles 1 to 25")),
			reply("I read 25 articles. Batches gives a Free limit that no plan has."),
			batchFix(article, "Fix the batch limit", "stale:batch", "- A batch can hold at most 1,000 shipments."),
			reply("I read 25 articles. One had a wrong limit. My suggestion is below."),
		)

		run := k.runRoutine(routine, true)

		run.AssertJSONPath("data.changesets_count", float64(1))
		run.AssertJSONPath("data.outcome", "findings")
		run.AssertJSONPath("data.steps.0.summary", "Started 1 subagent")
		run.AssertJSONPath("data.steps.1.tool", "subagent")
		run.AssertJSONPath("data.steps.1.summary", "Articles 1 to 25")
		run.AssertJSONPath("data.steps.1.subagent", float64(1))
		run.AssertJSONPath("data.steps.2.tool", "subagent_report")
		run.AssertJSONPath("data.steps.2.subagent", float64(1))
		run.AssertJSONPath("data.steps.3.summary", "1 subagent reported")
		run.AssertJSONPath("data.steps.4.summary", "Proposed: Fix the batch limit")
		sent := k.llm.Sent()
		assert.Contains(t, sent[0].System[0].Text, "subagents with run_subagents")
		assert.Contains(t, sent[1].System[0].Text, "You are a subagent")
		assert.Equal(t, "Audit the help articles at offset 0, limit 25.", sent[1].Messages[0].Parts[0].Text)
		assert.Contains(t, toolNames(sent[0]), "run_subagents")
		for _, name := range []string{"run_subagents", "save_routine_state", "report_nothing_to_change"} {
			assert.NotContains(t, toolNames(sent[1]), name)
		}
		for _, name := range []string{"propose_article_edits", "propose_new_article", "propose_changes"} {
			assert.Contains(t, toolNames(sent[0]), name)
			assert.NotContains(t, toolNames(sent[1]), name)
		}
		assert.Contains(t, sent[1].System[0].Text, "You suggest nothing")
		back := sent[2].Messages[len(sent[2].Messages)-1].Parts[0].Text
		assert.Contains(t, back, "Batches gives a Free limit that no plan has.")
		assert.Contains(t, back, "No subagent suggested anything")
	})

	t.Run("waits for every subagent and keeps each one's steps under its number", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			tools(split("Articles 1 to 25", "Articles 26 to 50")),
			reply("Nothing needed changing."),
			reply("Nothing needed changing."),
			tools(call("report_nothing_to_change", map[string]any{"reason": "All 50 hold up."})),
			reply("I read 50 articles. Nothing to change."),
		)

		run := k.runRoutine(routine, true)

		run.AssertJSONPath("data.outcome", "noop")
		body := run.String("data.steps")
		assert.Equal(t, 2, strings.Count(body, `"tool":"subagent_report"`))
		assert.Contains(t, body, `"subagent":2`)
		assert.Contains(t, body, "2 subagents reported")
	})

	t.Run("lists a subagent's run under the routine's thread, named as a subagent", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			tools(split("Articles 1 to 25")),
			reply("Nothing needed changing."),
			reply("I read 25 articles. Nothing to change."),
		)

		k.runRoutine(routine, true)

		usage := k.d.Owner.GetJSON("/api/v1/operator/usage").AssertOK()
		usage.AssertJSONMissing("data.recent.2")
		titles := []string{usage.String("data.recent.0.thread_title"), usage.String("data.recent.1.thread_title")}
		assert.Equal(t, usage.String("data.recent.0.thread_id"), usage.String("data.recent.1.thread_id"))
		assert.NotEmpty(t, usage.String("data.recent.0.thread_id"))
		assert.Equal(t, routine, usage.String("data.recent.0.routine_id"))
		assert.Equal(t, routine, usage.String("data.recent.1.routine_id"))
		assert.Contains(t, strings.Join(titles, "|"), " · subagent 1")
	})

	t.Run("refuses a split wider than a call or a run allows, and one from a subagent", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			tools(split("A", "B", "C", "D", "E")),
			tools(split("A")),
			tools(split("B")),
			reply("Could not split."),
			reply("Done."),
		)

		k.runRoutine(routine, true)

		sent := k.llm.Sent()
		wide := sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text
		assert.Contains(t, wide, `"subagents" must hold 1 to 4 entries`)
		nested := sent[3].Messages[len(sent[3].Messages)-1].Parts[0].Text
		assert.Contains(t, nested, "unknown tool run_subagents")
	})
}
