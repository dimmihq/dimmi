/*
|--------------------------------------------------------------------------
| Simulation History
|--------------------------------------------------------------------------
|
| An agent's answers vary from one run to the next, so a simulation is
| judged by how it has gone lately and not by its last run alone. Every
| run is kept, and the simulation carries a short history of the latest
| ones on each copy of the agent: passed four of the last five.
|
| Only runs that tested what is in front of the team now count. Each
| run is kept with a fingerprint of what it tested: the agent's copy,
| the workspace's procedures and the simulation itself. When any of
| them changes, the history starts again, so a score never rests on an
| agent or a check that no longer exists. The older runs are still
| kept, and can be read.
|
| A run that could not be played to its end, such as when the simulated
| customer gave no answer, says nothing about the agent: it is kept but
| counts for nothing.
|
*/

package agentsims

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
History sizes: the runs a score looks at, the runs of one simulation
kept on each copy, and the past runs listed at once.
*/
const (
	HistoryWindow = 5
	keptRuns      = 50
	listedRuns    = 20
)

/*
History is a simulation's latest runs on one copy of the agent, newest
first, all of them on what Fingerprint names.
*/
type History struct {
	Fingerprint string       `json:"fingerprint"`
	Runs        []HistoryRun `json:"runs"`
}

/*
HistoryRun is one run that counts: which, whether it passed and when.
*/
type HistoryRun struct {
	ID     ids.UUID  `json:"id"`
	Passed bool      `json:"passed"`
	RanAt  time.Time `json:"ran_at"`
}

/*
ReadHistory reads a stored history; nil when there is none.
*/
func ReadHistory(raw json.RawMessage) *History {
	var h History
	if len(raw) == 0 || json.Unmarshal(raw, &h) != nil || len(h.Runs) == 0 {
		return nil
	}
	return &h
}

/*
tested is what a run on each copy of an agent tests, apart from the
simulation itself: empty for a copy the agent does not have.
*/
type tested struct {
	draft string
	live  string
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
testedNow reads what a run on each copy of the agent would test now:
the working copy or the moment the live version was published, and the
state of the workspace's procedures, which both copies follow.
*/
func testedNow(ctx context.Context, tx *database.Tx, agent models.AiAgent) (tested, error) {
	q := models.New(tx)
	var written string
	err := tx.QueryRow(ctx, `SELECT count(*)::text || ':' || coalesce(max(updated_at)::text, '')
		FROM procedures WHERE workspace_id = $1 AND deleted_at IS NULL`, agent.WorkspaceID).Scan(&written)
	if err != nil {
		return tested{}, fmt.Errorf("read procedures state: %w", err)
	}
	placements, err := agents.Placements(ctx, q, agent.WorkspaceID)
	if err != nil {
		return tested{}, err
	}
	working, err := agents.Working(agent, placements[agent.ID])
	if err != nil {
		return tested{}, err
	}
	draft, _ := json.Marshal(working)
	out := tested{draft: "draft:" + string(draft) + written}
	if agent.Status != "draft" && agent.PublishedAt != nil {
		out.live = "live:" + strconv.FormatInt(agent.PublishedAt.UnixNano(), 10) + written
	}
	return out, nil
}

/*
fingerprint names what one run tests: the agent's copy and the
simulation as it stands.
*/
func fingerprint(agent string, c models.AiAgentTestCase) string {
	if agent == "" {
		return ""
	}
	parts := [][]byte{[]byte(agent), c.Persona, c.Messages, c.ActionMocks, c.Assertions, []byte(c.Scenario)}
	var all []byte
	for _, p := range parts {
		all = append(all, p...)
		all = append(all, 0)
	}
	return hashing.SHA256Hex(all)
}

/*
current blanks the histories that no longer speak for what a run would
test now, so a simulation whose agent or checks changed reads as not
run yet.
*/
func current(t tested, rows []models.AiAgentTestCase) {
	for i, c := range rows {
		if h := ReadHistory(c.DraftHistory); h == nil || h.Fingerprint != fingerprint(t.draft, c) {
			rows[i].DraftHistory = nil
		}
		if h := ReadHistory(c.LiveHistory); h == nil || h.Fingerprint != fingerprint(t.live, c) {
			rows[i].LiveHistory = nil
		}
	}
}

/*
fresh blanks stale histories on rows read outside a transaction.
*/
func (s *Simulations) fresh(ctx context.Context, m authz.Membership, agentID ids.UUID, rows []models.AiAgentTestCase,
) error {
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		agent, err := findAgent(ctx, models.New(tx), m.Workspace.ID, agentID)
		if err != nil {
			return err
		}
		t, err := testedNow(ctx, tx, agent)
		if err != nil {
			return err
		}
		current(t, rows)
		return nil
	})
}

/*
keptResult is how one run went, on which copy.
*/
type keptResult struct {
	version string
	result  procedures.TestResult
}

/*
keep saves how a simulation's run went on the copy it ran against, the
run itself and the simulation's history there.
*/
func (s *Simulations) keep(ctx context.Context, m authz.Membership, c models.AiAgentTestCase, k keptResult,
) (models.AiAgentTestCase, error) {
	raw, _ := json.Marshal(k.result)
	var history json.RawMessage
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		ran := pipeline.SimulationSpend{WorkspaceID: m.Workspace.ID, At: s.Clock.Now(), Runs: 1}
		if err := pipeline.AddSimulationSpend(ctx, tx, ran); err != nil {
			return err
		}
		var err error
		if history, err = record(ctx, tx, c, k); err != nil {
			return err
		}
		if k.version == VersionLive {
			return q.SetAgentTestCaseLiveResult(ctx, models.SetAgentTestCaseLiveResultParams{
				Result: raw, History: history, WorkspaceID: m.Workspace.ID, ID: c.ID,
			})
		}
		return q.SetAgentTestCaseDraftResult(ctx, models.SetAgentTestCaseDraftResultParams{
			Result: raw, History: history, WorkspaceID: m.Workspace.ID, ID: c.ID,
		})
	})
	if err != nil {
		return c, fmt.Errorf("keep simulation result: %w", err)
	}
	if k.version == VersionLive {
		c.LiveResult, c.LiveHistory = raw, history
	} else {
		c.DraftResult, c.DraftHistory = raw, history
	}
	return c, nil
}

/*
record keeps one run of a simulation and returns the simulation's
history on that copy with the run in it. A run that tested something
else than the history's runs did starts the history again; one that
could not be played to its end leaves it as it was.
*/
func record(ctx context.Context, tx *database.Tx, c models.AiAgentTestCase, k keptResult) (json.RawMessage, error) {
	q := models.New(tx)
	agent, err := findAgent(ctx, q, c.WorkspaceID, c.AgentID)
	if err != nil {
		return nil, err
	}
	t, err := testedNow(ctx, tx, agent)
	if err != nil {
		return nil, err
	}
	if k.version != VersionLive {
		k.version = VersionDraft
	}
	mark := fingerprint(t.of(k.version), c)
	raw, _ := json.Marshal(k.result)
	errored := k.result.Error != ""
	run, err := q.InsertAgentTestCaseRun(ctx, models.InsertAgentTestCaseRunParams{
		WorkspaceID: c.WorkspaceID, TestCaseID: c.ID, Version: k.version, Fingerprint: mark,
		Passed: k.result.Passed && !errored, Errored: errored, Result: raw, RanAt: k.result.RanAt,
	})
	if err != nil {
		return nil, fmt.Errorf("keep simulation run: %w", err)
	}
	err = q.PruneAgentTestCaseRuns(ctx, models.PruneAgentTestCaseRunsParams{
		WorkspaceID: c.WorkspaceID, TestCaseID: c.ID, Version: k.version, Keep: keptRuns,
	})
	if err != nil {
		return nil, fmt.Errorf("prune simulation runs: %w", err)
	}
	stored := c.DraftHistory
	if k.version == VersionLive {
		stored = c.LiveHistory
	}
	if errored {
		return stored, nil
	}
	h := ReadHistory(stored)
	if h == nil || h.Fingerprint != mark {
		h = &History{Fingerprint: mark}
	}
	h.Runs = append([]HistoryRun{{ID: run.ID, Passed: run.Passed, RanAt: run.RanAt}}, h.Runs...)
	h.Runs = h.Runs[:min(len(h.Runs), HistoryWindow)]
	return json.Marshal(h)
}

/*
PastRun is one kept run of a simulation. Counted says it tested what a
run would test now, so it is one a score may rest on.
*/
type PastRun struct {
	models.AiAgentTestCaseRun

	Counted bool
}

/*
Runs lists a simulation's latest runs on one copy of the agent, newest
first, whatever they tested.
*/
func (s *Simulations) Runs(ctx context.Context, m authz.Membership, ref SimulationRef, version string,
) ([]PastRun, error) {
	if err := ai.RequireAgentRead(m); err != nil {
		return nil, err
	}
	if version != VersionLive {
		version = VersionDraft
	}
	var out []PastRun
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := findSimulation(ctx, q, m.Workspace.ID, ref)
		if err != nil {
			return err
		}
		agent, err := findAgent(ctx, q, m.Workspace.ID, ref.Agent)
		if err != nil {
			return err
		}
		t, err := testedNow(ctx, tx, agent)
		if err != nil {
			return err
		}
		rows, err := q.ListAgentTestCaseRuns(ctx, models.ListAgentTestCaseRunsParams{
			WorkspaceID: m.Workspace.ID, TestCaseID: c.ID, Version: version, RowLimit: listedRuns,
		})
		if err != nil {
			return fmt.Errorf("list simulation runs: %w", err)
		}
		now := fingerprint(t.of(version), c)
		for _, r := range rows {
			out = append(out, PastRun{AiAgentTestCaseRun: r, Counted: !r.Errored && now != "" && r.Fingerprint == now})
		}
		return nil
	})
	return out, err
}
