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

	"github.com/dimmihq/dimmi/app/actions/agentsims"
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
plays, the messages, the mocks and the checks, how it last went on the
draft and on the live version, null until it ran there, and how its
latest runs there went, null until one counts.
*/
type AIAgentSimulation struct {
	ID                   string                    `json:"id"`
	Object               string                    `json:"object"`
	AgentID              string                    `json:"agent_id"`
	Name                 string                    `json:"name"`
	SourceConversationID *string                   `json:"source_conversation_id"`
	Persona              json.RawMessage           `json:"persona"`
	Messages             json.RawMessage           `json:"messages"`
	ActionMocks          json.RawMessage           `json:"action_mocks"`
	Assertions           json.RawMessage           `json:"assertions"`
	Scenario             string                    `json:"scenario"`
	DraftResult          json.RawMessage           `json:"draft_result"`
	LiveResult           json.RawMessage           `json:"live_result"`
	DraftHistory         *AIAgentSimulationHistory `json:"draft_history"`
	LiveHistory          *AIAgentSimulationHistory `json:"live_history"`
	SimulatedAt          *time.Time                `json:"simulated_at"`
	CreatedAt            time.Time                 `json:"created_at"`
	UpdatedAt            time.Time                 `json:"updated_at"`
}

/*
AIAgentSimulationHistory is how a simulation's latest runs went on one
copy of the agent as it stands now: passing when every one passed,
failing when none did and flaky in between, how many passed of how
many, and the runs themselves, newest first. Runs made before the
agent's copy, the procedures or the simulation changed are left out.
*/
type AIAgentSimulationHistory struct {
	State  string                        `json:"state" enum:"passing,flaky,failing"`
	Passed int                           `json:"passed"`
	Total  int                           `json:"total"`
	Window int                           `json:"window"`
	Runs   []AIAgentSimulationHistoryRun `json:"runs"`
}

/*
AIAgentSimulationHistoryRun is one run a history counts.
*/
type AIAgentSimulationHistoryRun struct {
	ID     string    `json:"id"`
	Passed bool      `json:"passed"`
	RanAt  time.Time `json:"ran_at"`
}

/*
newSimulationHistory shapes a stored history; nil when no run counts.
*/
func newSimulationHistory(raw json.RawMessage) *AIAgentSimulationHistory {
	h := agentsims.ReadHistory(raw)
	if h == nil {
		return nil
	}
	out := &AIAgentSimulationHistory{
		Total: len(h.Runs), Window: agentsims.HistoryWindow, Runs: make([]AIAgentSimulationHistoryRun, len(h.Runs)),
	}
	for i, r := range h.Runs {
		out.Runs[i] = AIAgentSimulationHistoryRun{
			ID: models.PublicID(models.PrefixAiAgentTestCaseRun, r.ID), Passed: r.Passed, RanAt: r.RanAt,
		}
		if r.Passed {
			out.Passed++
		}
	}
	switch out.Passed {
	case out.Total:
		out.State = "passing"
	case 0:
		out.State = "failing"
	default:
		out.State = "flaky"
	}
	return out
}

/*
AIAgentSimulationRun is one kept run of a simulation: which copy of the
agent it ran on, how it went, and whether it counts toward the
simulation's history, which only runs on the agent, procedures and
simulation as they stand now do.
*/
type AIAgentSimulationRun struct {
	ID           string          `json:"id"`
	Object       string          `json:"object"`
	SimulationID string          `json:"simulation_id"`
	Version      string          `json:"version" enum:"draft,live"`
	Passed       bool            `json:"passed"`
	Errored      bool            `json:"errored"`
	Counted      bool            `json:"counted"`
	Result       json.RawMessage `json:"result"`
	RanAt        time.Time       `json:"ran_at"`
}

/*
NewAIAgentSimulationRuns shapes a simulation's kept runs.
*/
func NewAIAgentSimulationRuns(rows []agentsims.PastRun) []AIAgentSimulationRun {
	out := make([]AIAgentSimulationRun, len(rows))
	for i, r := range rows {
		out[i] = AIAgentSimulationRun{
			ID: models.PublicID(models.PrefixAiAgentTestCaseRun, r.ID), Object: "ai_agent_simulation_run",
			SimulationID: models.PublicID(models.PrefixAiAgentTestCase, r.TestCaseID), Version: r.Version,
			Passed: r.Passed, Errored: r.Errored, Counted: r.Counted, Result: r.Result, RanAt: r.RanAt,
		}
	}
	return out
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
		Scenario:    c.Scenario,
		DraftResult: jsonOrNull(c.DraftResult), LiveResult: jsonOrNull(c.LiveResult), SimulatedAt: c.SimulatedAt,
		DraftHistory: newSimulationHistory(c.DraftHistory), LiveHistory: newSimulationHistory(c.LiveHistory),
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
