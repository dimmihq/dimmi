package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestOperatorTraces(t *testing.T) {
	t.Parallel()

	t.Run("a question is one Operator run with its model and tool calls beneath it", func(t *testing.T) {
		t.Parallel()
		tracker := tests.NewTracker(t)
		k := newDesk(t, tracker.Option())
		thread := k.thread(k.conversation("I was charged twice for March."))
		k.llm.Script(tools(call("get_conversation_by_number", map[string]any{})), reply("Refund one charge."))

		k.ask(thread, "What does Ana want?").AssertOK()
		spans := k.d.TC.TrackedSpans(tracker)

		threadID, err := k.d.TC.App.IDs.Decode(string(models.PrefixOperatorThread), thread)
		require.NoError(t, err)
		var runs, calls, used []tests.TrackedSpan
		for _, s := range spans {
			switch s.Kind() {
			case "agent":
				runs = append(runs, s)
			case "ai_client":
				calls = append(calls, s)
			case "tool":
				used = append(used, s)
			}
		}
		require.Len(t, runs, 1)
		run := runs[0]
		assert.Empty(t, run.ParentID, "the run is the trace's root")
		assert.Equal(t, "Operator", run.Attr("gen_ai.agent.name"))
		assert.Equal(t, "operator", run.Attr("gen_ai.agent.id"))
		assert.Equal(t, threadID.String(), run.Attr("gen_ai.conversation.id"))
		assert.Empty(t, run.Status)
		require.Len(t, calls, 2)
		for _, c := range calls {
			assert.Equal(t, run.SpanID, c.ParentID)
		}
		require.Len(t, used, 1)
		assert.Equal(t, "get_conversation_by_number", used[0].Attr("gen_ai.tool.name"))
		assert.Equal(t, run.SpanID, used[0].ParentID)
	})

	t.Run("a run the model fails is a failed Operator run", func(t *testing.T) {
		t.Parallel()
		tracker := tests.NewTracker(t)
		k := newDesk(t, tracker.Option())
		thread := k.thread("")
		k.llm.Script(fakeFailure())

		k.ask(thread, "Summarize the week")
		var failed []string
		for _, s := range k.d.TC.TrackedSpans(tracker) {
			if s.Kind() == "agent" {
				failed = append(failed, s.Attr("gen_ai.agent.name")+" "+s.Status)
			}
		}

		assert.Equal(t, []string{"Operator internal_error"}, failed)
	})
}

/*
The tracker counts a tool call as failed by its status alone: set and
not "ok" (apps/server/src/services/span.rs reduce_latency_groups and
agent_summary); error.type names the failure for whoever reads it.
*/
func TestOperatorToolFailureTraces(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		call    contracts.ToolCall
		status  string
		errType string
	}{
		{"a tool Operator does not have", call("delete_everything", map[string]any{}), "not_found", "unknown_tool"},
		{
			"a conversation that does not exist", call("get_conversation_by_number", map[string]any{"number": 999999}),
			"not_found", "not_found",
		},
		{
			"arguments of the wrong type", call("get_conversation_by_number", map[string]any{"number": "first"}),
			"invalid_argument", "invalid_arguments",
		},
		{"a tool that works", call("get_conversation_by_number", map[string]any{}), "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tracker := tests.NewTracker(t)
			k := newDesk(t, tracker.Option())
			thread := k.thread(k.conversation("I was charged twice for March."))
			k.llm.Script(tools(tc.call), reply("Done."))

			k.ask(thread, "What does Ana want?").AssertOK()

			var tool, run tests.TrackedSpan
			for _, s := range k.d.TC.TrackedSpans(tracker) {
				switch s.Kind() {
				case "tool":
					tool = s
				case "agent":
					run = s
				}
			}
			require.NotEmpty(t, tool.SpanID)
			assert.Equal(t, tc.status, tool.Status)
			assert.Equal(t, tc.errType, tool.Attr("error.type"))
			assert.Empty(t, run.Status, "Operator answered, so its run did not fail")
		})
	}
}

func TestOperatorRunKindTraces(t *testing.T) {
	t.Parallel()

	t.Run("a routine is an Operator run, and each subagent a run beneath it", func(t *testing.T) {
		t.Parallel()
		tracker := tests.NewTracker(t)
		k := newDesk(t, tracker.Option())
		routine := k.routine(testkit.JSON{"name": "Audit", "prompt": "Audit articles."})
		k.llm.Script(
			tools(split("Articles 1 to 25")),
			reply("I read 25 articles. Nothing is wrong."),
			reply("Nothing to change."),
		)

		k.runRoutine(routine, true)
		spans := k.d.TC.TrackedSpans(tracker)

		byID := map[string]tests.TrackedSpan{}
		var main, sub tests.TrackedSpan
		for _, s := range spans {
			byID[s.SpanID] = s
			switch {
			case s.Kind() == "agent" && s.Attr("gen_ai.agent.name") == "Operator":
				main = s
			case s.Kind() == "agent" && s.Attr("gen_ai.agent.name") == "Operator subagent":
				sub = s
			}
		}
		require.NotEmpty(t, main.SpanID, "the routine's run")
		require.NotEmpty(t, sub.SpanID, "the subagent's run")
		assert.Equal(t, "routine", main.Attr("dimmi.run.kind"))
		assert.Equal(t, "subagent", sub.Attr("dimmi.run.kind"))
		assert.Equal(t, "operator.subagent", sub.Attr("gen_ai.agent.id"))
		inside := false
		for at, ok := byID[sub.ParentID]; ok; at, ok = byID[at.ParentID] {
			inside = inside || at.SpanID == main.SpanID
		}
		assert.True(t, inside, "the subagent runs inside the routine's run")
		assert.Empty(t, main.Status)
		assert.Empty(t, sub.Status)
	})
}
