package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
spansOf picks the spans the tracker classifies as one kind.
*/
func spansOf(spans []tests.TrackedSpan, kind string) []tests.TrackedSpan {
	var out []tests.TrackedSpan
	for _, s := range spans {
		if s.Kind() == kind {
			out = append(out, s)
		}
	}
	return out
}

/*
agentOf returns the name of the agent run a span sits under, walking
its parents the way the tracker's trace view does.
*/
func agentOf(spans []tests.TrackedSpan, span tests.TrackedSpan) string {
	byID := map[string]tests.TrackedSpan{}
	for _, s := range spans {
		byID[s.SpanID] = s
	}
	for at, ok := span, true; ok; at, ok = byID[at.ParentID] {
		if at.Kind() == "agent" {
			return at.Attr("gen_ai.agent.name")
		}
	}
	return ""
}

func TestAgentTraces(t *testing.T) {
	t.Parallel()

	t.Run("a turn that searches and transfers is one trace of two named agent runs", func(t *testing.T) {
		t.Parallel()
		tracker := tests.NewTracker(t)
		r := newTransferDesk(t, tracker.Option())
		support := models.PublicID(models.PrefixAiAgent, r.support)
		r.llm.answer.Script(
			search("invoice export broken"),
			tools(call("transfer_agent", `{"agent_id":"`+support+`","summary":"Existing customer with a bug"}`)),
			reply("Sorry about the export. Which file type were you exporting?"),
		)
		conv, trigger := r.start("We're customers and the invoice export is broken")

		r.run(conv, trigger)
		spans := r.d.TC.TrackedSpans(tracker)

		// The root the two runs share is plain work, so the tracker
		// counts nothing for it.
		var root tests.TrackedSpan
		for _, s := range spans {
			if s.Transaction == "answer conversation" && s.ParentID == "" {
				root = s
			}
		}
		require.NotEmpty(t, root.SpanID, "one trace for the turn")
		assert.Equal(t, "function", root.Op)
		assert.Empty(t, root.Kind())

		// Agent runs: services/span.rs agent_summary counts
		// gen_ai_operation_type = 'agent'; trace_agent_names lists their
		// gen_ai.agent.name; agent_conversations groups by
		// gen_ai.conversation.id.
		runs := spansOf(spans, "agent")
		require.Len(t, runs, 2)
		assert.Equal(t, "Sales", runs[0].Attr("gen_ai.agent.name"))
		assert.Equal(t, r.sales.String(), runs[0].Attr("gen_ai.agent.id"))
		assert.Equal(t, "Ada", runs[1].Attr("gen_ai.agent.name"))
		assert.Equal(t, r.support.String(), runs[1].Attr("gen_ai.agent.id"))
		for _, run := range runs {
			assert.Equal(t, conv.String(), run.Attr("gen_ai.conversation.id"))
			assert.Equal(t, root.SpanID, run.ParentID, "both runs sit in the turn's trace")
			assert.Empty(t, run.Status, "neither run failed")
		}

		// Tool calls: gen_ai.tool.name, under the agent that made them.
		// Each agent searches the knowledge before it writes; Sales then
		// searches as scripted and transfers.
		var used []string
		for _, s := range spansOf(spans, "tool") {
			used = append(used, agentOf(spans, s)+" "+s.Attr("gen_ai.tool.name"))
		}
		assert.ElementsMatch(t, []string{
			"Sales search_knowledge", "Sales search_knowledge", "Sales transfer_agent", "Ada search_knowledge",
		}, used)

		// Model calls: every one sits under an agent run, none unnamed.
		calls := spansOf(spans, "ai_client")
		require.NotEmpty(t, calls)
		for _, c := range calls {
			assert.NotEmpty(t, agentOf(spans, c), "model call %s has an agent", c.Description)
			assert.NotEmpty(t, c.Attr("gen_ai.request.model"))
		}

		// Handoff: conversation.ts handoffsOf draws it from the agent the
		// span names to the next agent run that starts after it.
		handoffs := spansOf(spans, "handoff")
		require.Len(t, handoffs, 1)
		assert.Equal(t, "Sales", handoffs[0].Attr("gen_ai.agent.name"))
		assert.Equal(t, "Ada", handoffs[0].Attr("dimmi.handoff.to.name"))
		assert.Equal(t, runs[0].SpanID, handoffs[0].ParentID)
		assert.LessOrEqual(t, handoffs[0].Start, runs[1].Start)
	})

	t.Run("a turn the model fails is a failed agent run", func(t *testing.T) {
		t.Parallel()
		tracker := tests.NewTracker(t)
		h := newHarness(t, nil, tracker.Option())
		h.llm.answer.Script(fake.Step{Err: &contracts.LLMError{Kind: contracts.LLMErrorAuth, HTTPStatus: 401}})
		conv, trigger := h.conversation("Where is my order?")

		require.Equal(t, "error", h.run(conv, trigger).Outcome)
		runs := spansOf(h.d.TC.TrackedSpans(tracker), "agent")

		require.Len(t, runs, 1)
		assert.Equal(t, "Ada", runs[0].Attr("gen_ai.agent.name"))
		assert.Equal(t, "internal_error", runs[0].Status)
	})
}
