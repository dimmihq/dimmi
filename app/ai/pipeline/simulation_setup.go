/*
|--------------------------------------------------------------------------
| Simulation Setup
|--------------------------------------------------------------------------
|
| What a simulated turn starts from: the agent that answers, the
| procedures it may follow with the handed-back state picked up, and
| the answers its actions give when the test gives none.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
simulatedAgent is the agent a simulated turn answers as: the one asked
for, in the copy asked for, or the workspace's default, with the
workspace's shared models folded in.
*/
func simulatedAgent(ctx context.Context, q *models.Queries, in SimulationInput) (models.AiAgent, error) {
	if in.AgentID == nil {
		return agents.DefaultAnswerer(ctx, q, in.WorkspaceID)
	}
	agent, err := q.FindAgentByID(ctx, models.FindAgentByIDParams{WorkspaceID: in.WorkspaceID, ID: *in.AgentID})
	if errors.Is(err, database.ErrNotFound) {
		return agent, ErrNoAgent
	}
	if err != nil {
		return agent, fmt.Errorf("find agent: %w", err)
	}
	if in.Agent != nil {
		agents.Apply(&agent, *in.Agent)
	}
	settings, err := agents.Shared(ctx, q, in.WorkspaceID)
	if err != nil {
		return agent, err
	}
	return agents.Answering(agent, settings), nil
}

/*
loadSimProcedures offers the live procedures that apply to the persona,
with the one under test in the definition given, and picks up the
state handed back.
*/
func (r *Runner) loadSimProcedures(ctx context.Context, q *models.Queries, e *env, sim *simulation) error {
	in := sim.in
	rows, err := q.ListLiveProcedures(ctx, in.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list procedures: %w", err)
	}
	if in.ProcedureID != (ids.UUID{}) {
		tested := liveProcedure{
			row: models.ListLiveProceduresRow{ID: in.ProcedureID, Name: in.Name}, def: in.Definition,
			version: in.Version,
		}
		sim.tested = tested.publicID()
		e.AllProcedures = append(e.AllProcedures, tested)
	}
	for _, row := range rows {
		if row.ID == in.ProcedureID {
			continue
		}
		if def, err := procedure.Parse(row.Live); err == nil {
			e.AllProcedures = append(e.AllProcedures, liveProcedure{row: row, def: def, version: row.Version})
		}
	}
	if e.Procedures, err = r.inScope(ctx, q, e, e.AllProcedures); err != nil {
		return err
	}
	if s := in.State; s != nil {
		sim.pending = slices.Clone(s.Pending)
	}
	resumeSimProcedure(e, in.State)
	return nil
}

/*
resumeSimProcedure picks the procedure the handed-back state says is
active back up where it left off.
*/
func resumeSimProcedure(e *env, s *SimState) {
	if s == nil || s.Status != procedureActive || s.ProcedureID == "" {
		return
	}
	for _, p := range e.AllProcedures {
		if p.publicID() == s.ProcedureID {
			outputs, _ := json.Marshal(s.Outputs)
			branches, _ := json.Marshal(cmpSides(s.Branches))
			e.ActiveProcedure = &activeProcedure{proc: p, state: models.ConversationProcedureState{
				Status: procedureActive, CalledActionIds: s.Called, CompletedStepIds: s.Completed,
				StepOutputs: outputs, Branches: branches,
			}}
		}
	}
}

/*
loadTestResponses answers every action the test gives no mock for with
the result of its last live test.
*/
func (r *Runner) loadTestResponses(ctx context.Context, q *models.Queries, ws ids.UUID, sim *simulation) error {
	rows, err := q.ListActionTestResponses(ctx, ws)
	if err != nil {
		return fmt.Errorf("list action test responses: %w", err)
	}
	for _, row := range rows {
		public := models.PublicID(models.PrefixAiAction, row.ID)
		if _, ok := sim.mocks[public]; !ok {
			sim.mocks[public] = actionkit.Mock{Status: 200, Body: row.LastTestResponse, Shaped: true}
		}
	}
	return nil
}
