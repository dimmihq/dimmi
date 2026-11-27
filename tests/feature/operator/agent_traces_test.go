package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
