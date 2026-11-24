/*
|--------------------------------------------------------------------------
| Workflow Simulation History
|--------------------------------------------------------------------------
|
| A simulation with an AI-played customer and agent does not go the same
| way every time, so it is judged by how it has gone lately, as an
| agent's is: every run is kept with a fingerprint of the copy of the
| workflow it ran on and of the simulation itself, and each copy keeps
| the latest runs that tested what is in front of the team now. A run
| that could not be played to its end is kept but counts for nothing.
|
*/

package workflowsims

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentsims"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
History sizes: the runs of one simulation kept on each copy, and the
past runs listed at once.
*/
const (
	keptRuns   = 50
	listedRuns = 20
)

/*
tested is what a run on each copy of a workflow tests: empty for a copy
the workflow does not have.
*/
type tested struct {
	draft string
	live  string
}

/*
testedNow reads what a run on each copy would test now: the draft's
hash, and the live version.
*/
func testedNow(wf models.Workflow) tested {
	out := tested{draft: "draft:" + wf.DraftHash}
	if wf.LiveVersionID != nil {
		out.live = "live:" + wf.LiveVersionID.String()
	}
	return out
}

/*
of is what a run on the version tests.
*/
func (t tested) of(version string) string {
	if version == VersionLive {
		return t.live
	}
	return t.draft
}

/*
fingerprint names what one run tests: the workflow's copy and the
simulation as it stands.
*/
func fingerprint(copy string, c models.WorkflowSimulation) string {
	if copy == "" {
		return ""
	}
	var all []byte
	for _, p := range [][]byte{[]byte(copy), c.Persona, c.Messages, c.ActionMocks, c.Assertions, []byte(c.Scenario)} {
		all = append(all, p...)
		all = append(all, 0)
	}
	return hashing.SHA256Hex(all)
}

/*
current blanks the histories that no longer speak for what a run would
test now.
*/
func current(t tested, rows []models.WorkflowSimulation) {
	for i, c := range rows {
		if h := agentsims.ReadHistory(c.DraftHistory); h == nil || h.Fingerprint != fingerprint(t.draft, c) {
			rows[i].DraftHistory = nil
		}
		if h := agentsims.ReadHistory(c.LiveHistory); h == nil || h.Fingerprint != fingerprint(t.live, c) {
			rows[i].LiveHistory = nil
		}
	}
}

/*
fresh blanks stale histories on rows read outside a transaction.
*/
func (s *Simulations) fresh(ctx context.Context, m authz.Membership, workflowID ids.UUID,
	rows []models.WorkflowSimulation,
) error {
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		wf, err := findWorkflow(ctx, models.New(tx), m.Workspace.ID, workflowID)
		if err != nil {
			return err
		}
		current(testedNow(wf), rows)
		return nil
	})
}

/*
keptResult is how one run went, on which copy, and what it tested.
*/
type keptResult struct {
	version string
	result  Result
	tested  string
}

/*
keep saves how a simulation's run went on the copy it ran against: the
run itself and the simulation's history there.
*/
func (s *Simulations) keep(ctx context.Context, m authz.Membership, c models.WorkflowSimulation, k keptResult,
) (models.WorkflowSimulation, error) {
	raw, _ := json.Marshal(k.result)
	var history json.RawMessage
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		ran := pipeline.SimulationSpend{WorkspaceID: m.Workspace.ID, At: s.Clock.Now(), Runs: 1}
		if err := pipeline.AddSimulationSpend(ctx, tx, ran); err != nil {
			return err
		}
		var err error
		if history, err = record(ctx, q, c, k, raw); err != nil {
			return err
		}
		if k.version == VersionLive {
			return q.SetWorkflowSimulationLiveResult(ctx, models.SetWorkflowSimulationLiveResultParams{
				Result: raw, History: history, WorkspaceID: m.Workspace.ID, ID: c.ID,
			})
		}
		return q.SetWorkflowSimulationDraftResult(ctx, models.SetWorkflowSimulationDraftResultParams{
			Result: raw, History: history, WorkspaceID: m.Workspace.ID, ID: c.ID,
		})
	})
	if err != nil {
		return c, fmt.Errorf("keep workflow simulation result: %w", err)
	}
	if k.version == VersionLive {
		c.LiveResult, c.LiveHistory = raw, history
	} else {
		c.DraftResult, c.DraftHistory = raw, history
	}
	return c, nil
}

/*
record keeps one run and returns the simulation's history on that copy
with the run in it.
*/
func record(ctx context.Context, q *models.Queries, c models.WorkflowSimulation, k keptResult, raw []byte,
) (json.RawMessage, error) {
	mark := fingerprint(k.tested, c)
	errored := k.result.Error != ""
	run, err := q.InsertWorkflowSimulationRun(ctx, models.InsertWorkflowSimulationRunParams{
		WorkspaceID: c.WorkspaceID, SimulationID: c.ID, Version: k.version, Fingerprint: mark,
		Passed: k.result.Passed && !errored, Errored: errored, Result: raw, RanAt: k.result.RanAt,
	})
	if err != nil {
		return nil, fmt.Errorf("keep workflow simulation run: %w", err)
	}
	err = q.PruneWorkflowSimulationRuns(ctx, models.PruneWorkflowSimulationRunsParams{
		WorkspaceID: c.WorkspaceID, SimulationID: c.ID, Version: k.version, Keep: keptRuns,
	})
	if err != nil {
		return nil, fmt.Errorf("prune workflow simulation runs: %w", err)
	}
	stored := c.DraftHistory
	if k.version == VersionLive {
		stored = c.LiveHistory
	}
	if errored {
		return stored, nil
	}
	h := agentsims.ReadHistory(stored)
	if h == nil || h.Fingerprint != mark {
		h = &agentsims.History{Fingerprint: mark}
	}
	h.Runs = append([]agentsims.HistoryRun{{ID: run.ID, Passed: run.Passed, RanAt: run.RanAt}}, h.Runs...)
	h.Runs = h.Runs[:min(len(h.Runs), agentsims.HistoryWindow)]
	return json.Marshal(h)
}

/*
PastRun is one kept run of a simulation. Counted says it tested what a
run would test now.
*/
type PastRun struct {
	models.WorkflowSimulationRun

	Counted bool
}

/*
Runs lists a simulation's latest runs on one copy of the workflow,
newest first, whatever they tested.
*/
func (s *Simulations) Runs(ctx context.Context, m authz.Membership, ref SimulationRef, version string,
) ([]PastRun, error) {
	if err := requireRead(m); err != nil {
		return nil, err
	}
	if version != VersionLive {
		version = VersionDraft
	}
	var out []PastRun
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := findWorkflow(ctx, q, m.Workspace.ID, ref.Workflow)
		if err != nil {
			return err
		}
		c, err := findSimulation(ctx, q, m.Workspace.ID, ref)
		if err != nil {
			return err
		}
		rows, err := q.ListWorkflowSimulationRuns(ctx, models.ListWorkflowSimulationRunsParams{
			WorkspaceID: m.Workspace.ID, SimulationID: c.ID, Version: version, RowLimit: listedRuns,
		})
		if err != nil {
			return fmt.Errorf("list workflow simulation runs: %w", err)
		}
		now := fingerprint(testedNow(wf).of(version), c)
		for _, r := range rows {
			out = append(out, PastRun{WorkflowSimulationRun: r, Counted: !r.Errored && now != "" && r.Fingerprint == now})
		}
		return nil
	})
	return out, err
}
