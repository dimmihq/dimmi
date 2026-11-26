package aiattributes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/models"
)

func TestApplyKeepsWhatTeammatesSet(t *testing.T) {
	t.Parallel()

	defs := []models.AiConversationAttribute{
		{Key: "issue", Label: "Issue type", DataType: TypeChoice, Options: []string{"Billing", "Bug"}},
		{Key: "plan", Label: "Plan", DataType: TypeText},
		{Key: "mood", Label: "Mood", DataType: TypeChoice, Options: []string{"calm", "upset"}},
	}
	kept := map[string]any{"issue": "Billing", "plan": "Pro", "mood": "calm"}
	at := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	sources := map[string]Source{
		"issue": {By: SetByMember, Name: "Ada", At: at},
		"plan":  {By: SetByWorkflow, Name: "Workflow: Triage", At: at},
		"mood":  {By: SetByAI, At: at},
	}

	next, changes := apply(defs, kept, sources, map[string]any{"issue": "Bug", "plan": "Team", "mood": "upset"}, SetByAI)
	assert.Equal(t, "Billing", next["issue"], "the AI never replaces a teammate's value")
	assert.Equal(t, "Pro", next["plan"], "nor a workflow's")
	assert.Equal(t, "upset", next["mood"], "but replaces its own")
	assert.Equal(t, []Change{{Key: "mood", Label: "Mood", From: "calm", To: "upset"}}, changes)

	next, changes = apply(defs, kept, sources, map[string]any{"issue": "Bug", "plan": nil}, SetByMember)
	assert.Equal(t, "Bug", next["issue"], "a teammate replaces anyone's value")
	assert.NotContains(t, next, "plan", "and clears one")
	assert.Len(t, changes, 2)

	_, changes = apply(defs, kept, sources, map[string]any{"issue": "Billing", "nope": "x"}, SetByMember)
	assert.Empty(t, changes, "the same value changes nothing")
}

func TestAIFillsOnlyWhatItCanRead(t *testing.T) {
	t.Parallel()

	for _, dataType := range []string{TypeText, TypeLongText, TypeNumber, TypeDecimal, TypeBoolean, TypeDate, TypeChoice} {
		assert.True(t, AIFillable(dataType), dataType)
	}

	for _, dataType := range []string{TypeDateTime, TypeMultipleChoice, TypeFile, TypeContact, TypeCompany} {
		assert.False(t, AIFillable(dataType), dataType)
	}

	assert.False(t, byAI(models.AiConversationAttribute{DataType: TypeText, FilledBy: FilledByTeammates}))
	assert.True(t, byAI(models.AiConversationAttribute{DataType: TypeText, FilledBy: FilledByBoth}))
}

func TestSureKeepsOnlyValuesThatFit(t *testing.T) {
	t.Parallel()

	defs := []models.AiConversationAttribute{
		{Key: "seats", Label: "Seats", DataType: TypeNumber},
		{Key: "refund", Label: "Refund", DataType: TypeDecimal},
		{Key: "issue", Label: "Issue", DataType: TypeChoice, Options: []string{"Billing"}},
	}

	got := sure(defs, map[string]any{"seats": 2.5, "refund": 2.5, "issue": "Other"})
	assert.Equal(t, map[string]any{"refund": 2.5}, got, "a fraction is no whole number, and a choice must be listed")
}
