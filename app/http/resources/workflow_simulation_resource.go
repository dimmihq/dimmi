/*
|--------------------------------------------------------------------------
| Workflow Simulation Resources
|--------------------------------------------------------------------------
|
| A workflow's saved simulations as the API reads them, with how each
| last went on the draft and on the live version, and a series' dry
| simulation of one person.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/agentsims"
	"github.com/dimmihq/dimmi/app/actions/workflowsims"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowSimulation is one saved simulation of a workflow: the customer
it plays, the messages, the mocks by step id and the checks, how it
last went on the draft and on the live version, null until it ran
there, and how its latest runs there went, null until one counts. A
result carries the workflow's trace: the trigger's verdict, each step
and path, the variables, the actions that would run, the chat and
dimmi's turns inside agent steps.
*/
type WorkflowSimulation struct {
	ID                   string                     `json:"id"`
	Object               string                     `json:"object"`
	WorkflowID           string                     `json:"workflow_id"`
	Name                 string                     `json:"name"`
	SourceConversationID *string                    `json:"source_conversation_id"`
	Persona              json.RawMessage            `json:"persona"`
	Messages             json.RawMessage            `json:"messages"`
	ActionMocks          json.RawMessage            `json:"action_mocks"`
	Assertions           json.RawMessage            `json:"assertions"`
	Scenario             string                     `json:"scenario"`
	DraftResult          json.RawMessage            `json:"draft_result"`
	LiveResult           json.RawMessage            `json:"live_result"`
	DraftHistory         *WorkflowSimulationHistory `json:"draft_history"`
	LiveHistory          *WorkflowSimulationHistory `json:"live_history"`
	SimulatedAt          *time.Time                 `json:"simulated_at"`
	CreatedAt            time.Time                  `json:"created_at"`
	UpdatedAt            time.Time                  `json:"updated_at"`
}

/*
WorkflowSimulationHistory is how a simulation's latest runs went on one
copy of the workflow as it stands now: passing when every one passed,
failing when none did and flaky in between.
*/
type WorkflowSimulationHistory struct {
	State  string                         `json:"state" enum:"passing,flaky,failing"`
	Passed int                            `json:"passed"`
	Total  int                            `json:"total"`
	Window int                            `json:"window"`
	Runs   []WorkflowSimulationHistoryRun `json:"runs"`
}

/*
WorkflowSimulationHistoryRun is one run a history counts.
*/
type WorkflowSimulationHistoryRun struct {
	ID     string    `json:"id"`
	Passed bool      `json:"passed"`
	RanAt  time.Time `json:"ran_at"`
}

/*
newWorkflowSimulationHistory shapes a stored history; nil when no run
counts.
*/
func newWorkflowSimulationHistory(raw json.RawMessage) *WorkflowSimulationHistory {
	h := agentsims.ReadHistory(raw)
	if h == nil {
		return nil
	}
	out := &WorkflowSimulationHistory{
		Total: len(h.Runs), Window: agentsims.HistoryWindow, Runs: make([]WorkflowSimulationHistoryRun, len(h.Runs)),
	}
	for i, r := range h.Runs {
		out.Runs[i] = WorkflowSimulationHistoryRun{
			ID: models.PublicID(models.PrefixWorkflowSimRun, r.ID), Passed: r.Passed, RanAt: r.RanAt,
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
NewWorkflowSimulation shapes one saved simulation.
*/
func NewWorkflowSimulation(c models.WorkflowSimulation) WorkflowSimulation {
	return WorkflowSimulation{
		ID: models.PublicID(models.PrefixWorkflowSim, c.ID), Object: "workflow_simulation",
		WorkflowID: models.PublicID(models.PrefixWorkflow, c.WorkflowID), Name: c.Name,
		SourceConversationID: publicPtr(models.PrefixConversation, c.SourceConversationID),
		Persona:              c.Persona, Messages: c.Messages, ActionMocks: c.ActionMocks, Assertions: c.Assertions,
		Scenario: c.Scenario, DraftResult: jsonOrNull(c.DraftResult), LiveResult: jsonOrNull(c.LiveResult),
		DraftHistory: newWorkflowSimulationHistory(c.DraftHistory),
		LiveHistory:  newWorkflowSimulationHistory(c.LiveHistory),
		SimulatedAt:  c.SimulatedAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

/*
NewWorkflowSimulations shapes a list of saved simulations.
*/
func NewWorkflowSimulations(rows []models.WorkflowSimulation) []WorkflowSimulation {
	out := make([]WorkflowSimulation, len(rows))
	for i, r := range rows {
		out[i] = NewWorkflowSimulation(r)
	}
	return out
}

/*
WorkflowSimulationRun is one kept run of a simulation: which copy of the
workflow it ran on, how it went, and whether it counts toward the
simulation's history.
*/
type WorkflowSimulationRun struct {
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
NewWorkflowSimulationRuns shapes a simulation's kept runs.
*/
func NewWorkflowSimulationRuns(rows []workflowsims.PastRun) []WorkflowSimulationRun {
	out := make([]WorkflowSimulationRun, len(rows))
	for i, r := range rows {
		out[i] = WorkflowSimulationRun{
			ID: models.PublicID(models.PrefixWorkflowSimRun, r.ID), Object: "workflow_simulation_run",
			SimulationID: models.PublicID(models.PrefixWorkflowSim, r.SimulationID), Version: r.Version,
			Passed: r.Passed, Errored: r.Errored, Counted: r.Counted, Result: r.Result, RanAt: r.RanAt,
		}
	}
	return out
}
