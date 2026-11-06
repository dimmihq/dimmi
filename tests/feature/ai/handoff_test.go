package ai_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/models"
)

/*
handoffNote is the four-line note the scripted agent writes.
*/
const handoffNote = "Wants: a refund of a double charge.\nTried: nothing yet.\nMood: upset.\n" +
	"Handed off because: refunds need a teammate."

/*
handoffCall is a scripted handoff tool call.
*/
func handoffCall(reason string) fake.Step {
	args, _ := json.Marshal(map[string]string{"reason": reason, "note_for_team": handoffNote})
	return tools(call("handoff", string(args)))
}

/*
TestAiHandoffs hands customers to the team whenever the agent or the
policy says so, and never leaves a customer it was serving in silence.
*/
func TestAiHandoffs(t *testing.T) {
	t.Parallel()

	t.Run("hands off with the agent's note, its reason and the configured routing", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		team := h.team("Billing")
		h.operator(`UPDATE ai_agents
			SET handoff = jsonb_build_object('target_team_id', $2::uuid, 'raise_priority', true)
			WHERE workspace_id = $1`, h.d.TC.Workspace.Workspace.ID, team.ID)
		h.llm.answer.Script(handoffCall("refund"), reply("I'm bringing in a teammate to sort out the refund."))

		conv, trigger := h.conversation("You charged me twice. Refund one now.")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "refund", res.HandoffReason)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "handed_off", s.conv.AiState)
		assert.Equal(t, "refund", str(s.conv.AiHandoffReason))
		require.NotNil(t, s.conv.AssigneeTeamID)
		assert.Equal(t, team.ID, *s.conv.AssigneeTeamID)
		assert.Positive(t, s.conv.Priority)
		require.Len(t, s.agentReplies(), 1)
		assert.Equal(t, "I'm bringing in a teammate to sort out the refund.", str(s.agentReplies()[0].Body))
		require.Len(t, s.notes(), 1)
		assert.Equal(t, handoffNote, str(s.notes()[0].Body))

		mode := h.llm.answer.Sent()[1].Messages[2].Parts[0].Text
		assert.JSONEq(t, `{"ok":true,"mode":"available"}`, mode)
	})

	t.Run("posts the default line when the model adds nothing after handing off", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(handoffCall("customer_request"), reply(""))

		conv, trigger := h.conversation("Get me a human")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Contains(t, str(s.agentReplies()[0].Body), "bringing in a teammate")
	})

	t.Run("marks the conversation for the team without offering a teammate when handoff is unavailable",
		func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, nil)
			h.operator("UPDATE ai_agents SET unknown_behavior = 'say_unknown' WHERE workspace_id = $1",
				h.d.TC.Workspace.Workspace.ID)
			h.llm.answer.Script(handoffCall("legal"), reply("This is something I can't resolve here."))

			conv, trigger := h.conversation("My lawyer will contact you")
			res := h.run(conv, trigger)
			assert.Equal(t, "handoff", res.Outcome)

			s := h.load(conv, res.RunID)
			assert.Equal(t, "handed_off", s.conv.AiState)
			assert.Equal(t, false, aiMeta(t, s.agentReplies()[0])["handoff_available"])
			assert.Contains(t, h.llm.answer.Sent()[1].Messages[2].Parts[0].Text, `"mode":"unavailable"`)
		})

	t.Run("hands off at the reply limit", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MaxAiReplies = 2 })

		conv, trigger := h.conversation("How much is Pro?")
		h.operator("UPDATE conversations SET ai_state = 'active', ai_reply_count = 2 WHERE id = $1", conv)

		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "max_replies", res.HandoffReason)
		assert.Empty(t, h.llm.answer.Sent())

		s := h.load(conv, res.RunID)
		assert.Contains(t, str(s.agentReplies()[0].Body), "reached what I can help with")
		require.Len(t, s.notes(), 1)
		assert.Contains(t, str(s.notes()[0].Body), "Handed off because: max replies.")
	})

	t.Run("hands off a customer it was serving when the spend cap is reached", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MonthlySpendCapMicros = new(int64(1000)) })
		h.spent(5000)

		conv, trigger := h.conversation("How much is Pro?")
		h.operator("UPDATE conversations SET ai_state = 'active' WHERE id = $1", conv)

		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "spend_cap", res.HandoffReason)
		assert.Empty(t, h.llm.answer.Sent())
	})

	t.Run("stays quiet for a new customer when the spend cap is reached", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MonthlySpendCapMicros = new(int64(1000)) })
		h.spent(5000)

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "spend_cap", res.SkipReason)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "skipped", s.run.Status)
		assert.Empty(t, s.agentReplies())
		assert.Equal(t, "off", s.conv.AiState)
	})

	t.Run("ignores a spend cap of zero", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MonthlySpendCapMicros = new(int64(0)) })
		h.llm.answer.Script(reply("Hi!"))

		conv, trigger := h.conversation("hello")
		assert.Equal(t, "greeted", h.run(conv, trigger).Outcome)
	})

	t.Run("hands off when the provider fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(fake.Step{Err: &contracts.LLMError{Kind: contracts.LLMErrorAuth, HTTPStatus: 401}})

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "error", res.Outcome)
		assert.Equal(t, "provider_error", res.HandoffReason)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "failed", s.run.Status)
		assert.Equal(t, "provider_error", str(s.run.ErrorCode))
		assert.Equal(t, "handed_off", s.conv.AiState)
		assert.Contains(t, str(s.agentReplies()[0].Body), "teammate")
		require.Len(t, s.notes(), 1)
	})

	t.Run("hands off without a model while serving the customer", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.AnswerModel = nil })

		conv, trigger := h.conversation("How much is Pro?")
		h.operator("UPDATE conversations SET ai_state = 'active' WHERE id = $1", conv)

		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "provider_unavailable", res.HandoffReason)
	})

	t.Run("hands off outside the schedule while serving the customer", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) {
			p.Schedule = "inside_office_hours"
			p.OfficeHours = json.RawMessage(`{"timezone":"UTC","days":{"sun":{"open":"00:00","close":"00:01"}}}`)
		})

		conv, trigger := h.conversation("How much is Pro?")
		h.operator("UPDATE conversations SET ai_state = 'active' WHERE id = $1", conv)

		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "schedule", res.HandoffReason)
	})

	t.Run("steps back when a teammate already replied", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)

		conv, trigger := h.conversation("How much is Pro?")
		h.add(conv, "member", "message", "Hi! Pro is $12 per seat.")

		res := h.run(conv, trigger)
		assert.Equal(t, "human_replied", res.SkipReason)
		assert.Empty(t, h.llm.answer.Sent())
	})
}
