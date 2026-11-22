/*
|--------------------------------------------------------------------------
| AI Agent Simulation Resources
|--------------------------------------------------------------------------
|
| An agent's preview turn and its saved simulations as the API reads
| them, with how each last went on the draft and on the live version.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
AIAgentPreview is one preview turn: the run that answered, the reply
and its outcome, the handover, the state to hand back, the procedure it
followed and the actions it called.
*/
type AIAgentPreview struct {
	Object         string                   `json:"object"`
	RunID          string                   `json:"run_id"`
	Reply          string                   `json:"reply"`
	Outcome        string                   `json:"outcome"`
	Handoff        *pipeline.SimHandoff     `json:"handoff"`
	ActiveState    pipeline.SimState        `json:"active_state"`
	ProcedureTrace json.RawMessage          `json:"procedure_trace,omitempty"`
	ActionCalls    []procedure.ObservedCall `json:"action_calls"`
	Transfer       *pipeline.SimTransfer    `json:"transfer"`
}

/*
NewAIAgentPreview shapes a preview turn.
*/
func NewAIAgentPreview(r *pipeline.SimulationResult) AIAgentPreview {
	return AIAgentPreview{
		Object: "ai_agent_preview", RunID: models.PublicID(models.PrefixAiRun, r.RunID), Reply: r.Reply,
		Outcome: r.Outcome, Handoff: r.Handoff, ActiveState: r.State, ProcedureTrace: r.ProcedureTrace,
		ActionCalls: r.Calls, Transfer: r.Transfer,
	}
}

/*
AIAgentSimulation is one saved simulation of an agent: the customer it
plays, the messages, the mocks and the checks, and how it last went on
the draft and on the live version, null until it ran there.
*/
type AIAgentSimulation struct {
	ID                   string          `json:"id"`
	Object               string          `json:"object"`
	AgentID              string          `json:"agent_id"`
	Name                 string          `json:"name"`
	SourceConversationID *string         `json:"source_conversation_id"`
	Persona              json.RawMessage `json:"persona"`
	Messages             json.RawMessage `json:"messages"`
	ActionMocks          json.RawMessage `json:"action_mocks"`
	Assertions           json.RawMessage `json:"assertions"`
	DraftResult          json.RawMessage `json:"draft_result"`
	LiveResult           json.RawMessage `json:"live_result"`
	SimulatedAt          *time.Time      `json:"simulated_at"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

/*
NewAIAgentSimulation shapes one saved simulation.
*/
func NewAIAgentSimulation(c models.AiAgentTestCase) AIAgentSimulation {
	return AIAgentSimulation{
		ID: models.PublicID(models.PrefixAiAgentTestCase, c.ID), Object: "ai_agent_simulation",
		AgentID: models.PublicID(models.PrefixAiAgent, c.AgentID), Name: c.Name,
		SourceConversationID: publicPtr(models.PrefixConversation, c.SourceConversationID),
		Persona:              c.Persona, Messages: c.Messages, ActionMocks: c.ActionMocks, Assertions: c.Assertions,
		DraftResult: jsonOrNull(c.DraftResult), LiveResult: jsonOrNull(c.LiveResult), SimulatedAt: c.SimulatedAt,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

/*
NewAIAgentSimulations shapes a list of saved simulations.
*/
func NewAIAgentSimulations(rows []models.AiAgentTestCase) []AIAgentSimulation {
	out := make([]AIAgentSimulation, len(rows))
	for i, r := range rows {
		out[i] = NewAIAgentSimulation(r)
	}
	return out
}

/*
jsonOrNull is a stored JSON value, or null when there is none.
*/
func jsonOrNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}
