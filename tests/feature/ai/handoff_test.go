package ai_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TestAiHandoffs hands customers to the team whenever the agent should
stop, and never leaves a customer it was serving in silence.
*/
func TestAiHandoffs(t *testing.T) {
	t.Parallel()

	t.Run("hands off at once when the customer asks for a person", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.cheap.Script(summaryStep())

		conv, trigger := h.conversation("I want to talk to a human please")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "customer_requested", res.HandoffReason)
		assert.Empty(t, h.llm.answer.Sent())

		s := h.load(conv, res.RunID)
		assert.Equal(t, "handed_off", s.conv.AiState)
		assert.Equal(t, "customer_requested", str(s.conv.AiHandoffReason))
		assert.Equal(t, []string{"summarize"}, s.stepKinds())
		require.Len(t, s.agentReplies(), 1)
		assert.Contains(t, str(s.agentReplies()[0].Body), "bringing in a teammate")
		require.Len(t, s.notes(), 1)
		assert.Contains(t, str(s.notes()[0].Body), "Wants: help.")
	})

	t.Run("recognizes explicit requests for a person in many languages", func(t *testing.T) {
		t.Parallel()
		for _, text := range []string{
			"human", "Operator!", "agent please", "Can I speak with a real person?", "talk to someone",
			"connect me to an agent", "Ich möchte mit einem Menschen sprechen", "Je veux parler à un conseiller",
			"Quiero hablar con un humano", "Quero falar com um atendente", "Vorrei parlare con un operatore",
			"Ik wil met een medewerker praten", "担当者と話したい",
		} {
			assert.True(t, pipeline.AsksForHuman(text), text)
		}
	})

	t.Run("leaves questions that only mention people to the classifier", func(t *testing.T) {
		t.Parallel()
		for _, text := range []string{
			"Can a staff member see my private tasks?", "Does the agent support Slack?",
			"How do I invite a person to my workspace?", "Is the human resources template free?",
			"My agent token expired",
		} {
			assert.False(t, pipeline.AsksForHuman(text), text)
		}

		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nYes, any project exports to CSV [1]."))

		conv, trigger := h.conversation("Can a staff member export a project to CSV?")
		assert.Equal(t, "answered", h.run(conv, trigger).Outcome)
	})

	t.Run("hands off when the classifier hears a request for a person", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("human_request", "en"))
		h.llm.cheap.Script(summaryStep())

		conv, trigger := h.conversation("Please let somebody from your company look at this")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "customer_requested", res.HandoffReason)
		assert.Empty(t, h.llm.answer.Sent())
	})

	t.Run("hands off at the reply limit", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MaxAiReplies = 2 })
		h.llm.cheap.Script(summaryStep())

		conv, trigger := h.conversation("How much is Pro?")
		h.operator("UPDATE conversations SET ai_state = 'active', ai_reply_count = 2 WHERE id = $1", conv)

		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "max_replies", res.HandoffReason)
		assert.Empty(t, h.llm.answer.Sent())

		s := h.load(conv, res.RunID)
		assert.Equal(t, "handed_off", s.conv.AiState)
		assert.Contains(t, str(s.agentReplies()[0].Body), "reached what I can help with")
	})

	t.Run("hands off a customer it was serving when the spend cap is reached", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MonthlySpendCapMicros = new(int64(1000)) })
		h.llm.cheap.Script(summaryStep())
		h.operator(`INSERT INTO ai_spend_periods (workspace_id, period_start, cost_micros, updated_at)
			VALUES ($1, date_trunc('month', $2::timestamptz AT TIME ZONE 'UTC')::date, 5000, now())`,
			h.d.TC.Workspace.Workspace.ID, h.d.TC.App.Clock.Now())

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
		h.operator(`INSERT INTO ai_spend_periods (workspace_id, period_start, cost_micros, updated_at)
			VALUES ($1, date_trunc('month', $2::timestamptz AT TIME ZONE 'UTC')::date, 5000, now())`,
			h.d.TC.Workspace.Workspace.ID, h.d.TC.App.Clock.Now())

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "skipped", res.Outcome)
		assert.Equal(t, "spend_cap", res.SkipReason)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "skipped", s.run.Status)
		assert.Empty(t, s.agentReplies())
		assert.Equal(t, "off", s.conv.AiState)
	})

	t.Run("ignores a spend cap of zero", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.MonthlySpendCapMicros = new(int64(0)) })
		h.decider.Script(decide("question", "en"))
		h.llm.answer.Script(reply("STATUS: answered\n\nPro costs $12 per seat per month [1]."))

		conv, trigger := h.conversation("How much is Pro?")
		assert.Equal(t, "answered", h.run(conv, trigger).Outcome)
	})

	t.Run("hands off when the provider fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.decider.Script(decide("question", "en"))
		h.llm.cheap.Script(summaryStep())
		h.llm.answer.Script(fake.Step{Err: &contracts.LLMError{Kind: contracts.LLMErrorServer, HTTPStatus: 503}})

		conv, trigger := h.conversation("How much is Pro?")
		res := h.run(conv, trigger)
		assert.Equal(t, "error", res.Outcome)
		assert.Equal(t, "provider_error", res.HandoffReason)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "failed", s.run.Status)
		assert.Equal(t, "provider_error", str(s.run.ErrorCode))
		assert.Equal(t, "handed_off", s.conv.AiState)
		require.Len(t, s.agentReplies(), 1)
		assert.Contains(t, str(s.agentReplies()[0].Body), "teammate")
		require.Len(t, s.notes(), 1)
	})

	t.Run("hands off without a model while serving the customer", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.AnswerModel = nil })
		h.llm.cheap.Script(summaryStep())

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
		h.llm.cheap.Script(summaryStep())

		conv, trigger := h.conversation("How much is Pro?")
		h.operator("UPDATE conversations SET ai_state = 'active' WHERE id = $1", conv)

		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "schedule", res.HandoffReason)
	})

	t.Run("posts the model's handoff and flags it when no human handoff is configured", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("ALTER TABLE ai_agents DROP CONSTRAINT IF EXISTS ai_agents_unknown_behavior_check")
		h.operator("UPDATE ai_agents SET unknown_behavior = 'say_unknown' WHERE workspace_id = $1",
			h.d.TC.Workspace.Workspace.ID)
		h.decider.Script(decide("question", "en"))
		h.llm.cheap.Script(summaryStep())
		h.llm.answer.Script(reply("STATUS: handoff\n\nThis is something I can't resolve here."))

		conv, trigger := h.conversation("Refund my annual plan now")
		res := h.run(conv, trigger)
		assert.Equal(t, "handoff", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "handed_off", s.conv.AiState)
		meta := aiMeta(t, s.agentReplies()[0])
		assert.Equal(t, false, meta["handoff_available"])
		assert.Contains(t, sentText(h.llm.answer), `Handoff is "unavailable" right now.`)
	})

	t.Run("steps back when a teammate already replied", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)

		conv, trigger := h.conversation("How much is Pro?")
		h.add(conv, "member", "message", "Hi! Pro is $12 per seat.")

		res := h.run(conv, trigger)
		assert.Equal(t, "skipped", res.Outcome)
		assert.Equal(t, "human_replied", res.SkipReason)
		assert.Empty(t, h.llm.cheap.Sent())
	})
}
