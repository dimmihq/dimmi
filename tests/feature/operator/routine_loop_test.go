package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

/*
TestOperatorRoutineLoop covers how a routine's run ends: what it may
still do once its reading is spent, the message it is asked for, and
where what it says while working goes.
*/
func TestOperatorRoutineLoop(t *testing.T) {
	t.Parallel()

	t.Run("lets a run only close once its reading is spent", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.setRoutine(routine, "max_tool_calls = 1")
		k.llm.Script(
			tools(call("list_articles", map[string]any{})),
			tools(call("list_articles", map[string]any{})),
			tools(call("mark_noop", map[string]any{"reason": "All fine"})),
			reply("Nothing to change."),
		)

		run := k.runRoutine(routine, true)

		run.AssertJSONPath("data.steps.1.ok", false)
		run.AssertJSONPath("data.steps.2.ok", true)
		sent := k.llm.Sent()
		assert.Len(t, sent[1].Tools, len(sent[0].Tools), "the tools stay the same so the prompt cache holds")
		refused := sent[2].Messages[len(sent[2].Messages)-1].Parts[0].Text
		assert.Contains(t, refused, "out of reading steps")
	})

	t.Run("asks for the message once a run may call no more tools", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.setRoutine(routine, "max_model_calls = 1")
		k.llm.Script(
			tools(call("list_articles", map[string]any{})),
			tools(call("list_articles", map[string]any{})),
			tools(call("list_articles", map[string]any{})),
			tools(call("list_articles", map[string]any{})),
			reply("I read the articles. Nothing to change."),
		)

		k.runRoutine(routine, true)

		sent := k.llm.Sent()
		last := sent[len(sent)-1]
		assert.Empty(t, last.Tools)
		assert.Contains(t, last.Messages[len(last.Messages)-1].Parts[0].Text, "You are out of steps")
		assert.Equal(t, 1, k.count(
			`SELECT count(*) FROM operator_messages WHERE body = 'I read the articles. Nothing to change.'`))
	})

	t.Run("keeps what the model says while it works as a step, not in the message", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			fake.Step{Text: "Reading them now.", ToolCalls: []contracts.ToolCall{
				call("list_articles", map[string]any{}),
			}},
			reply("I read the articles. Nothing to change."),
		)

		run := k.runRoutine(routine, true)

		run.AssertJSONPath("data.steps.0.tool", "note")
		run.AssertJSONPath("data.steps.0.summary", "Reading them now.")
		run.AssertJSONPath("data.steps.1.tool", "list_articles")
		assert.Equal(t, 1, k.count(
			`SELECT count(*) FROM operator_messages WHERE body = 'I read the articles. Nothing to change.'`))
	})
}
