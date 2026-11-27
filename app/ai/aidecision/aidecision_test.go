package aidecision_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/decision/fake"
)

/*
model is a scripted chat model: each call takes the next answer, and
an answer that is an error fails the call.
*/
type model struct {
	answers []any
	asked   []aidecision.ModelQuestion
}

func (m *model) AskModel(_ context.Context, q aidecision.ModelQuestion, out any) error {
	m.asked = append(m.asked, q)
	if len(m.answers) == 0 {
		return errors.New("no answer scripted")
	}

	next := m.answers[0]
	m.answers = m.answers[1:]

	if err, ok := next.(error); ok {
		return err
	}

	raw, _ := json.Marshal(next)

	return json.Unmarshal(raw, out)
}

func decider(t *testing.T, steps ...fake.Step) *fake.Decider {
	t.Helper()

	d, err := fake.New("testing")
	require.NoError(t, err)

	return d.Script(steps...)
}

func choice() aidecision.Choice {
	return aidecision.Choice{
		Subject:      aidecision.Subject{Context: "Customer: I was charged twice"},
		Instructions: "Sort by topic.",
		Options: []aidecision.Option{
			{ID: "p_billing", Label: "Billing", Description: "Invoices and refunds"},
			{ID: "p_bugs", Label: "Bugs"},
		},
	}
}

func TestChoose(t *testing.T) {
	t.Parallel()

	t.Run("takes the decision model's choice", func(t *testing.T) {
		t.Parallel()
		d := decider(t, fake.Step{Answers: map[string]contracts.DecisionAnswer{
			"path": {Type: contracts.DecisionChoice, Choice: "p_billing", Confidence: 0.9},
		}})
		m := &model{}

		got, err := aidecision.New(d, m).Choose(t.Context(), choice())
		require.NoError(t, err)
		assert.Equal(t, aidecision.Chosen{OptionID: "p_billing", Confidence: "high", By: aidecision.ByDecision}, got)
		assert.Empty(t, m.asked, "the chat model is not asked")

		sent := d.Sent()
		require.Len(t, sent, 1)
		assert.Contains(t, sent[0].State, "<untrusted>")
		assert.Contains(t, sent[0].Questions["path"].Choices, "none")
	})

	t.Run("reads none as no match", func(t *testing.T) {
		t.Parallel()
		d := decider(t, fake.Step{Answers: map[string]contracts.DecisionAnswer{
			"path": {Choice: "none", Confidence: 0.7},
		}})

		got, err := aidecision.New(d, nil).Choose(t.Context(), choice())
		require.NoError(t, err)
		assert.Empty(t, got.OptionID)
		assert.Equal(t, "medium", got.Confidence)
	})

	t.Run("asks the decision model again, then the chat model", func(t *testing.T) {
		t.Parallel()
		d := decider(t, fake.Step{Err: errors.New("down")})
		m := &model{answers: []any{map[string]string{"choice": "p_bugs", "confidence": "low"}}}

		got, err := aidecision.New(d, m).Choose(t.Context(), choice())
		require.NoError(t, err)
		assert.Len(t, d.Sent(), 2)
		assert.Equal(t, aidecision.Chosen{OptionID: "p_bugs", Confidence: "low", By: aidecision.ByModel}, got)
		require.Len(t, m.asked, 1)
		assert.Contains(t, m.asked[0].System, "Sort by topic.")
		assert.Contains(t, m.asked[0].System, "p_billing: Billing. Invoices and refunds")
	})

	t.Run("asks again about a choice off the list instead of keeping it", func(t *testing.T) {
		t.Parallel()
		d := decider(t,
			fake.Step{Answers: map[string]contracts.DecisionAnswer{"path": {Choice: "p_other", Confidence: 1}}},
			fake.Step{Answers: map[string]contracts.DecisionAnswer{"path": {Choice: "p_bugs", Confidence: 0.6}}},
		)

		got, err := aidecision.New(d, nil).Choose(t.Context(), choice())
		require.NoError(t, err)
		assert.Equal(t, "p_bugs", got.OptionID)
	})

	t.Run("keeps how sure the decision model was and the chat model's reasons", func(t *testing.T) {
		t.Parallel()
		d := decider(t, fake.Step{Answers: map[string]contracts.DecisionAnswer{
			"refund": {Choice: "yes", Confidence: 0.95},
			"reason": {Choice: "damaged", Confidence: 0.9},
		}})
		m := &model{answers: []any{map[string]any{
			"order": "1234", "amount": nil, "when": nil,
			"_reasons": map[string]any{"order": "The customer names order 1234.", "amount": nil, "when": nil},
		}}}

		e := extraction()
		e.Explain = true
		e.Fields[4].OptionDescriptions = map[string]string{"damaged": "Arrived broken or faulty"}

		got, err := aidecision.New(d, m).Extract(t.Context(), e)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"order": "The customer names order 1234."}, got.Reasons)
		assert.InDelta(t, 0.95, got.Sureness["refund"], 0.001)
		assert.InDelta(t, 0.9, got.Sureness["reason"], 0.001)
		assert.Contains(t, string(m.asked[0].Schema), `"_reasons"`)
	})

	t.Run("asks for no reasons unless told to", func(t *testing.T) {
		t.Parallel()
		m := &model{answers: []any{map[string]any{
			"order": "1234", "amount": nil, "when": nil, "refund": nil, "reason": nil,
		}}}

		_, err := aidecision.New(nil, m).Extract(t.Context(), extraction())
		require.NoError(t, err)
		assert.NotContains(t, string(m.asked[0].Schema), `"_reasons"`)
	})

	t.Run("says no model answered", func(t *testing.T) {
		t.Parallel()
		m := &model{answers: []any{errors.New("down")}}

		_, err := aidecision.New(nil, m).Choose(t.Context(), choice())
		require.ErrorIs(t, err, aidecision.ErrUnavailable)

		_, err = aidecision.New(nil, nil).Choose(t.Context(), choice())
		require.ErrorIs(t, err, aidecision.ErrUnavailable)
	})
}

func extraction() aidecision.Extraction {
	return aidecision.Extraction{
		Subject: aidecision.Subject{Context: "Customer: order 1234 arrived broken on 2026-11-20, I want a refund"},
		Fields: []aidecision.Field{
			{Key: "order", Type: aidecision.TypeText},
			{Key: "amount", Type: aidecision.TypeNumber},
			{Key: "when", Type: aidecision.TypeDate},
			{Key: "refund", Type: aidecision.TypeBoolean},
			{Key: "reason", Type: aidecision.TypeChoice, Options: []string{"damaged", "late", "other"}},
		},
	}
}

func TestExtract(t *testing.T) {
	t.Parallel()

	t.Run("asks the decision model for choices and the chat model for words", func(t *testing.T) {
		t.Parallel()
		d := decider(t, fake.Step{Answers: map[string]contracts.DecisionAnswer{
			"refund": {Choice: "yes", Confidence: 0.95},
			"reason": {Choice: "damaged", Confidence: 0.9},
		}})
		m := &model{answers: []any{map[string]any{"order": "1234", "amount": nil, "when": "2026-11-20"}}}

		got, err := aidecision.New(d, m).Extract(t.Context(), extraction())
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"order": "1234", "when": "2026-11-20", "refund": true, "reason": "damaged"},
			got.Values)
		assert.Equal(t, []string{"amount"}, got.Unsure)

		require.Len(t, m.asked, 1)
		var schema struct {
			Required []string `json:"required"`
		}
		require.NoError(t, json.Unmarshal(m.asked[0].Schema, &schema))
		assert.ElementsMatch(t, []string{"order", "amount", "when"}, schema.Required)
	})

	t.Run("leaves an unsure choice to the chat model", func(t *testing.T) {
		t.Parallel()
		d := decider(t, fake.Step{Answers: map[string]contracts.DecisionAnswer{
			"refund": {Choice: "yes", Confidence: 0.3},
			"reason": {Choice: "unknown", Confidence: 0.9},
		}})
		m := &model{answers: []any{map[string]any{
			"order": "1234", "amount": nil, "when": nil, "refund": true,
		}}}

		got, err := aidecision.New(d, m).Extract(t.Context(), extraction())
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"order": "1234", "refund": true}, got.Values)
		assert.ElementsMatch(t, []string{"amount", "when", "reason"}, got.Unsure)
	})

	t.Run("asks again about a value in the wrong shape, never fixing it", func(t *testing.T) {
		t.Parallel()
		m := &model{answers: []any{
			map[string]any{
				"order": "1234", "amount": nil, "when": "20 November", "refund": true, "reason": "damaged",
			},
			map[string]any{"when": "next week"},
		}}

		got, err := aidecision.New(nil, m).Extract(t.Context(), extraction())
		require.NoError(t, err)
		assert.NotContains(t, got.Values, "when")
		assert.Contains(t, got.Unsure, "when")
		require.Len(t, m.asked, 2)
		assert.Contains(t, string(m.asked[1].Schema), `"when"`)
		assert.NotContains(t, string(m.asked[1].Schema), `"order"`)
	})

	t.Run("says no model answered", func(t *testing.T) {
		t.Parallel()
		got, err := aidecision.New(nil, &model{answers: []any{errors.New("down")}}).Extract(t.Context(), extraction())
		require.ErrorIs(t, err, aidecision.ErrUnavailable)
		assert.Len(t, got.Unsure, 5)
	})
}

func TestSureEnough(t *testing.T) {
	t.Parallel()

	assert.True(t, aidecision.SureEnough("high", "medium"))
	assert.False(t, aidecision.SureEnough("low", "medium"))
	assert.True(t, aidecision.SureEnough("low", ""))
}
