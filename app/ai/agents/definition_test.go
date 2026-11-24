package agents_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/models"
)

func published() models.AiAgent {
	key := ids.New()
	model := "claude-test"
	return models.AiAgent{
		Name: "dimmi", Tone: "friendly", AnswerKeyID: &key, AnswerModel: &model,
		EnabledChannels: []string{"widget"}, Languages: []string{}, LinkAllowlist: []string{},
		ContactAttributeAllowlist: []string{}, Handoff: json.RawMessage(`{}`), Messages: json.RawMessage(`{}`),
		KnowledgeScope: json.RawMessage(`{"mode": "all"}`), ProcedureScope: json.RawMessage(`{"mode":"all"}`),
		ActionScope: json.RawMessage(`{"mode":"all"}`), TransferAgentIds: []ids.UUID{},
	}
}

func TestWorkingKeepsWhatTheDraftCleared(t *testing.T) {
	agent := published()
	def := agents.Published(agent, agents.Placement{})
	def.AnswerKeyID, def.AnswerModel = nil, nil
	agent.Draft, _ = json.Marshal(def)

	working, err := agents.Working(agent, agents.Placement{})
	require.NoError(t, err)

	assert.Nil(t, working.AnswerKeyID)
	assert.Nil(t, working.AnswerModel)
}

func TestSameIgnoresHowTheJSONIsSpaced(t *testing.T) {
	agent := published()
	a := agents.Published(agent, agents.Placement{})
	agent.Handoff = json.RawMessage(`{ }`)
	b := agents.Published(agent, agents.Placement{})

	assert.True(t, agents.Same(a, b))
	b.Tone = "concise"
	assert.False(t, agents.Same(a, b))
}

func TestScopes(t *testing.T) {
	allowed, other := ids.New(), ids.New()
	agent := published()
	agent.ProcedureScope = json.RawMessage(`{"mode":"selected","ids":["` + allowed.String() + `"]}`)
	agent.ActionScope = json.RawMessage(`not json`)

	assert.True(t, agents.ProceduresOf(agent).Allows(allowed))
	assert.False(t, agents.ProceduresOf(agent).Allows(other))
	assert.True(t, agents.ActionsOf(agent).Allows(other), "an unreadable scope allows everything")
	assert.Equal(t, agents.ScopeAll, agents.KnowledgeOf(agent).Mode)
}

func TestAnswersOn(t *testing.T) {
	conn := ids.New()
	agent := published()
	agent.Enabled, agent.IsDefault = true, true

	assert.True(t, agents.AnswersOn(agent, "widget", conn))
	assert.False(t, agents.AnswersOn(agent, "email", conn), "the default answers only on channels it ticked")
	agent.ExcludedChannelConnectionIds = []ids.UUID{conn}
	assert.False(t, agents.AnswersOn(agent, "widget", conn))

	agent.IsDefault = false
	assert.True(t, agents.AnswersOn(agent, "email", conn), "a placed agent answers where it is placed")
	agent.Enabled = false
	assert.False(t, agents.AnswersOn(agent, "email", conn))
}

func TestAnsweringFoldsInTheWorkspaceSettings(t *testing.T) {
	agent := published()
	agent.AnswerKeyID, agent.AnswerModel = nil, nil
	key, model, capMicros := ids.New(), "gpt-test", int64(5)
	s := models.WorkspaceAiSetting{AnswerKeyID: &key, AnswerModel: &model, MonthlySpendCapMicros: &capMicros}

	out := agents.Answering(agent, s)

	assert.Equal(t, &key, out.AnswerKeyID)
	assert.Equal(t, &model, out.AnswerModel)
	assert.Equal(t, &capMicros, out.MonthlySpendCapMicros)
	own := published()
	assert.Equal(t, own.AnswerModel, agents.Answering(own, s).AnswerModel, "an agent's own model wins")
}
