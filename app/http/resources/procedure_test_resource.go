/*
|--------------------------------------------------------------------------
| Procedure Test Resources
|--------------------------------------------------------------------------
|
| How the simulator and saved tests read over the API. A simulated turn
| answers with the reply, the outcome and any handover, the action calls
| it made (mocked, never run), the procedure's trace, and the state to
| send back with the next turn. A saved test carries its persona,
| scripted messages, mocks, assertions and how its last run went.
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
ProcedureSimulation is one simulated turn.
*/
type ProcedureSimulation struct {
	Object         string                   `json:"object"`
	RunID          string                   `json:"run_id"`
	Reply          string                   `json:"reply"`
	Outcome        string                   `json:"outcome"`
	Matched        bool                     `json:"matched"`
	Handoff        *pipeline.SimHandoff     `json:"handoff"`
	ActiveState    pipeline.SimState        `json:"active_state"`
	ProcedureTrace json.RawMessage          `json:"procedure_trace,omitempty"`
	ActionCalls    []procedure.ObservedCall `json:"action_calls"`
}

/*
NewProcedureSimulation shapes a simulated turn.
*/
func NewProcedureSimulation(r *pipeline.SimulationResult) ProcedureSimulation {
	return ProcedureSimulation{
		Object: "procedure_simulation", RunID: models.PublicID(models.PrefixAiRun, r.RunID), Reply: r.Reply,
		Outcome: r.Outcome, Matched: r.Matched, Handoff: r.Handoff, ActiveState: r.State,
		ProcedureTrace: r.ProcedureTrace, ActionCalls: r.Calls,
	}
}

/*
ProcedureTestCase is one saved test of a procedure.
*/
type ProcedureTestCase struct {
	ID                   string          `json:"id"`
	Object               string          `json:"object"`
	ProcedureID          string          `json:"procedure_id"`
	Name                 string          `json:"name"`
	SourceConversationID *string         `json:"source_conversation_id"`
	Persona              json.RawMessage `json:"persona"`
	Messages             json.RawMessage `json:"messages"`
	ActionMocks          json.RawMessage `json:"action_mocks"`
	Assertions           json.RawMessage `json:"assertions"`
	Scenario             string          `json:"scenario"`
	LastResult           json.RawMessage `json:"last_result"`
	SimulatedAt          *time.Time      `json:"simulated_at"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

/*
NewProcedureTestCase shapes one saved test.
*/
func NewProcedureTestCase(c models.ProcedureTestCase) ProcedureTestCase {
	last := c.LastResult
	if len(last) == 0 {
		last = json.RawMessage("null")
	}

	return ProcedureTestCase{
		ID: models.PublicID(models.PrefixProcedureTest, c.ID), Object: "procedure_test_case",
		ProcedureID: models.PublicID(models.PrefixProcedure, c.ProcedureID), Name: c.Name,
		SourceConversationID: publicPtr(models.PrefixConversation, c.SourceConversationID),
		Persona:              c.Persona, Messages: c.Messages, ActionMocks: c.ActionMocks, Assertions: c.Assertions,
		Scenario:   c.Scenario,
		LastResult: last, SimulatedAt: c.SimulatedAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

/*
NewProcedureTestCases shapes a list of saved tests.
*/
func NewProcedureTestCases(rows []models.ProcedureTestCase) []ProcedureTestCase {
	out := make([]ProcedureTestCase, len(rows))
	for i, r := range rows {
		out[i] = NewProcedureTestCase(r)
	}

	return out
}
