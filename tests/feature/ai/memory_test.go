package ai_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

/*
memoryRow is the conversation's stored memory.
*/
type memoryRow struct {
	Summary   string `json:"summary"`
	ThroughID string `json:"through_id"`
	Folds     int    `json:"folds"`
	Passages  []struct {
		ChunkID string `json:"chunk_id"`
		Title   string `json:"title"`
	} `json:"passages"`
}

/*
memoryOf reads the conversation's memory.
*/
func (h *harness) memoryOf(conv ids.UUID) memoryRow {
	h.t.Helper()
	var raw []byte
	require.NoError(h.t, h.d.TC.Operator().QueryRow(h.t.Context(),
		"SELECT coalesce(ai_memory, '{}'::jsonb) FROM conversations WHERE id = $1", conv).Scan(&raw))
	var m memoryRow
	require.NoError(h.t, json.Unmarshal(raw, &m))
	return m
}

/*
longThread writes n exchanges after the opening message, the first
naming an order number the summary must keep.
*/
func (h *harness) longThread(conv ids.UUID, n int) {
	h.t.Helper()
	for i := range n {
		h.add(conv, "agent", "message", fmt.Sprintf("Answer number %d.", i+1))
		h.add(conv, "contact", "message", fmt.Sprintf("Question number %d about ORD-1234.", i+2))
	}
}

/*
summarized is the scripted summary the answer model writes.
*/
func summarized(text string) fake.Step {
	return fake.Step{Text: text, Usage: contracts.LLMUsage{InputTokens: 700, OutputTokens: 60, Reported: true}}
}

func TestConversationMemory(t *testing.T) {
	t.Parallel()

	t.Run("folds the oldest turns into a summary after the reply and reads it on the next one", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, _ := h.conversation("My order ORD-1234 has not shipped. Question number 1.")
		h.longThread(conv, 8)
		trigger := h.add(conv, "contact", "message", "How much is Pro per seat?")

		h.llm.answer.Script(search("pro plan price per seat"), reply("Pro costs $12 per seat per month [1]."),
			summarized("- Wants: order ORD-1234 shipped.\n- Asked 8 questions about it."))
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		m := h.memoryOf(conv)
		assert.Contains(t, m.Summary, "ORD-1234")
		assert.NotEmpty(t, m.ThroughID)
		assert.Equal(t, 1, m.Folds)

		s := h.load(conv, res.RunID)
		assert.Equal(t, []string{"generate", "tool_call", "generate", "summarize"}, s.stepKinds())
		var stepsCost int64
		for _, st := range s.steps {
			stepsCost += st.CostMicros
		}
		assert.Equal(t, stepsCost, s.run.CostMicros, "the summary's cost lands on the run")
		assert.Equal(t, s.run.CostMicros, s.conv.AiCostMicros, "and on the conversation")

		summary := h.llm.answer.Sent()[2]
		assert.Contains(t, summary.Messages[0].Parts[0].Text, "My order ORD-1234 has not shipped")
		assert.NotContains(t, summary.Messages[0].Parts[0].Text, "How much is Pro",
			"the newest turns stay word for word")

		next := h.add(conv, "contact", "message", "And for Business?")
		h.llm.answer.Script(search("business plan price"), reply("Business costs $20 per seat [1]."))
		res = h.run(conv, next)
		require.Equal(t, "answered", res.Outcome)

		sent := h.llm.answer.Sent()
		first := sent[len(sent)-2].Messages[0].Parts[0].Text
		assert.True(t, strings.HasPrefix(first, "<earlier_conversation>- Wants: order ORD-1234 shipped."), first)
		var all strings.Builder
		for _, msg := range sent[len(sent)-2].Messages {
			all.WriteString(msg.Parts[0].Text)
		}
		assert.NotContains(t, all.String(), "My order ORD-1234 has not shipped",
			"folded turns are read from the summary only")
		assert.Contains(t, all.String(), "How much is Pro per seat?")
		assert.Equal(t, 1, h.memoryOf(conv).Folds, "no new fold until another block builds up")
	})

	t.Run("keeps the plain window when the summary cannot be written", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, _ := h.conversation("My order ORD-1234 has not shipped. Question number 1.")
		h.longThread(conv, 8)
		trigger := h.add(conv, "contact", "message", "How much is Pro per seat?")

		h.llm.answer.Script(search("pro plan price per seat"), reply("Pro costs $12 per seat per month [1]."),
			fake.Step{Err: &contracts.LLMError{Kind: contracts.LLMErrorAuth, HTTPStatus: 401}})
		res := h.run(conv, trigger)

		require.Equal(t, "answered", res.Outcome, "the reply stands")
		assert.Empty(t, h.memoryOf(conv).Summary)
	})

	t.Run("carries the cited passages so the next reply can answer without searching", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("UPDATE ai_agents SET carry_passages = true WHERE workspace_id = $1", h.d.TC.Workspace.Workspace.ID)
		conv, trigger := h.conversation("How much is Pro per seat?")

		h.llm.answer.Script(search("pro plan price per seat"), reply("Pro costs $12 per seat per month [1]."))
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)
		m := h.memoryOf(conv)
		require.Len(t, m.Passages, 1)
		assert.Empty(t, m.Summary, "a short conversation keeps no summary")

		next := h.add(conv, "contact", "message", "So 3 seats on Pro?")
		h.llm.answer.Script(reply("That is 3 times $12 per seat each month [1]."))
		res := h.run(conv, next)

		require.Equal(t, "answered", res.Outcome)
		sent := h.llm.answer.Sent()
		first := sent[len(sent)-1].Messages[0].Parts[0].Text
		assert.Contains(t, first, "<known_passages>\n[1] "+m.Passages[0].Title)
		replies := h.load(conv, res.RunID).agentReplies()
		meta := aiMeta(t, replies[len(replies)-1])
		sources, _ := meta["sources"].([]any)
		assert.Len(t, sources, 1, "the carried passage is cited like a search result")
	})

	t.Run("hands the conversation over once the agent's spend on it reaches the cap", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, trigger := h.conversation("How much is Pro?")
		h.operator("UPDATE conversations SET ai_state = 'active', ai_reply_count = 3, ai_cost_micros = 100000 "+
			"WHERE id = $1", conv)

		res := h.run(conv, trigger)

		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "conversation_cost", res.HandoffReason)
		assert.Empty(t, h.llm.answer.Sent())
	})
}
