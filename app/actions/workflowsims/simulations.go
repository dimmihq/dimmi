/*
|--------------------------------------------------------------------------
| Workflow Simulations
|--------------------------------------------------------------------------
|
| A teammate tries a workflow the way a customer would meet it before
| anyone does: a saved simulation plays a made-up customer, or one made
| from a real conversation, through the workflow's draft or its live
| version. The customer is played by AI: it taps buttons, answers what
| collect steps ask, lets timeouts pass, and writes during a wait or
| lets it run out. dimmi's turns inside agent steps run on the agent
| simulator, and a called workflow's steps run inline. Nothing reaches
| a customer or the team's systems: the workflow's actions are only
| recorded, and its HTTP requests and actions answer with their mocks.
|
| Each run keeps its trace: whether the trigger would start the
| workflow, every step and path, the variables, the actions that would
| run and the agent's turns. Its checks are judged as an agent's are,
| and each copy keeps a short history, so draft and live can be
| compared before publishing.
|
*/

package workflowsims

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	engine "github.com/dimmihq/dimmi/app/workflows"
)

/*
Which copy of a workflow a simulation runs against.
*/
const (
	VersionDraft = "draft"
	VersionLive  = "live"
)

/*
Simulation limits: the saved simulations a workflow keeps and runs at
once, and the customer messages a simulation made from a conversation
keeps.
*/
const (
	maxSimulations       = 100
	maxSimulationsAtOnce = 20
	maxImportedMessages  = 12
)

/*
The failures of workflow simulations.
*/
var (
	ErrSimulationNotFound = fault.New(fault.NotFound, "workflow_simulation_not_found",
		"This workflow has no simulation with this id.")
	ErrWorkflowNotFound = fault.New(fault.NotFound, "workflow_not_found", "No workflow of this workspace has this id.")
	ErrWorkflowNotLive  = fault.New(fault.Invalid, "workflow_not_live", "This workflow has no live version.").
				OnField("version")
	ErrSimulatorOff = fault.New(fault.Unavailable, "simulator_unavailable",
		"The simulator is not available on this server.")
	ErrSimulationInvalid = fault.New(fault.Invalid, "workflow_simulation_invalid",
		"The simulation cannot be saved as it is.")
)

/*
DryRunner walks a workflow's definition without a run.
*/
type DryRunner interface {
	/*
		DryRun walks the definition and returns its trace.
	*/
	DryRun(ctx context.Context, in engine.DryRun) (*engine.DryTrace, error)
}

/*
Simulations runs workflows' saved simulations.

Rules:
  - Reading needs workflows:read; saving, changing and running
    simulations workflows:manage.

Errors: [ErrWorkflowNotFound], [ErrSimulationNotFound],
[ErrWorkflowNotLive], [ErrSimulatorOff], a [fault] naming the field it
refuses, or a wrapped database error.
*/
type Simulations struct {
	DB         *database.DB
	Clock      clock.Clock
	Engine     DryRunner
	Simulator  procedures.Simulator
	Procedures *procedures.Procedures
}

/*
SimulationRef names one saved simulation of a workflow.
*/
type SimulationRef struct {
	Workflow   ids.UUID
	Simulation ids.UUID
}

/*
requireRead refuses a member who may not read workflows.
*/
func requireRead(m authz.Membership) error {
	if !m.Can("workflows:read") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
requireManage refuses a member who may not manage workflows.
*/
func requireManage(m authz.Membership) error {
	if !m.Can("workflows:manage") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
List returns a workflow's saved simulations, oldest first.
*/
func (s *Simulations) List(ctx context.Context, m authz.Membership, workflowID ids.UUID,
) ([]models.WorkflowSimulation, error) {
	if err := requireRead(m); err != nil {
		return nil, err
	}
	var out []models.WorkflowSimulation
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := findWorkflow(ctx, q, m.Workspace.ID, workflowID)
		if err != nil {
			return err
		}
		out, err = q.ListWorkflowSimulations(ctx, models.ListWorkflowSimulationsParams{
			WorkspaceID: m.Workspace.ID, WorkflowID: workflowID,
		})
		if err != nil {
			return fmt.Errorf("list workflow simulations: %w", err)
		}
		current(testedNow(wf), out)
		return nil
	})
	return out, err
}

/*
Show returns one saved simulation.
*/
func (s *Simulations) Show(ctx context.Context, m authz.Membership, ref SimulationRef,
) (models.WorkflowSimulation, error) {
	if err := requireRead(m); err != nil {
		return models.WorkflowSimulation{}, err
	}
	var out models.WorkflowSimulation
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := findWorkflow(ctx, q, m.Workspace.ID, ref.Workflow)
		if err != nil {
			return err
		}
		if out, err = findSimulation(ctx, q, m.Workspace.ID, ref); err != nil {
			return err
		}
		one := []models.WorkflowSimulation{out}
		current(testedNow(wf), one)
		out = one[0]
		return nil
	})
	return out, err
}

/*
Create saves a simulation of a workflow.
*/
func (s *Simulations) Create(ctx context.Context, m authz.Membership, workflowID ids.UUID,
	in procedures.TestCaseInput,
) (models.WorkflowSimulation, error) {
	if err := requireManage(m); err != nil {
		return models.WorkflowSimulation{}, err
	}
	if in.Name == nil {
		return models.WorkflowSimulation{}, ErrSimulationInvalid.With("reason", "Name the simulation.").OnField("name")
	}
	if err := checkAssertions(in.Assertions); err != nil {
		return models.WorkflowSimulation{}, err
	}
	if s.Procedures != nil {
		in = s.Procedures.WithScenario(ctx, m, in)
	}
	var out models.WorkflowSimulation
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		room := newSimulation{ws: m.Workspace.ID, workflow: workflowID, source: in.SourceConversation}
		if err := roomFor(ctx, q, room); err != nil {
			return err
		}
		row, err := build(models.ProcedureTestCase{}, in)
		if err != nil {
			return err
		}
		out, err = q.CreateWorkflowSimulation(ctx, models.CreateWorkflowSimulationParams{
			WorkspaceID: m.Workspace.ID, WorkflowID: workflowID, Name: row.Name,
			SourceConversationID: in.SourceConversation, Persona: row.Persona, Messages: row.Messages,
			ActionMocks: row.ActionMocks, Assertions: row.Assertions, SimulatedAt: row.SimulatedAt,
			Scenario: row.Scenario, MemberID: memberOf(m), Now: s.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("save workflow simulation: %w", err)
		}
		return nil
	})
	return out, err
}

/*
FromConversation saves a real conversation as a simulation: its
customer messages, in order, become the script, and the dry run reads
the conversation's own facts.
*/
func (s *Simulations) FromConversation(ctx context.Context, m authz.Membership, workflowID, conversation ids.UUID,
) (models.WorkflowSimulation, error) {
	if err := requireManage(m); err != nil {
		return models.WorkflowSimulation{}, err
	}
	if s.Procedures == nil {
		return models.WorkflowSimulation{}, ErrSimulatorOff
	}
	script, err := s.Procedures.ReadConversation(ctx, m, conversation)
	if err != nil {
		return models.WorkflowSimulation{}, err
	}
	var messages []procedures.TestMessage
	for _, msg := range script {
		if msg.From == "customer" && len(messages) < maxImportedMessages {
			messages = append(messages, procedures.TestMessage{Text: msg.Text})
		}
	}
	if len(messages) == 0 {
		return models.WorkflowSimulation{}, ErrSimulationInvalid.
			With("reason", "The conversation has no customer message to replay.").OnField("conversation_id")
	}
	name := clipName("From a conversation: " + messages[0].Text)
	return s.Create(ctx, m, workflowID, procedures.TestCaseInput{
		Name: &name, SourceConversation: &conversation, Messages: messages,
	})
}

/*
Update saves a changed simulation.
*/
func (s *Simulations) Update(ctx context.Context, m authz.Membership, ref SimulationRef,
	in procedures.TestCaseInput,
) (models.WorkflowSimulation, error) {
	if err := requireManage(m); err != nil {
		return models.WorkflowSimulation{}, err
	}
	if err := checkAssertions(in.Assertions); err != nil {
		return models.WorkflowSimulation{}, err
	}
	var out models.WorkflowSimulation
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := findWorkflow(ctx, q, m.Workspace.ID, ref.Workflow)
		if err != nil {
			return err
		}
		cur, err := findSimulation(ctx, q, m.Workspace.ID, ref)
		if err != nil {
			return err
		}
		row, err := build(asCase(cur), in)
		if err != nil {
			return err
		}
		out, err = q.UpdateWorkflowSimulation(ctx, models.UpdateWorkflowSimulationParams{
			WorkspaceID: m.Workspace.ID, WorkflowID: ref.Workflow, ID: ref.Simulation, Name: row.Name,
			Persona: row.Persona, Messages: row.Messages, ActionMocks: row.ActionMocks, Assertions: row.Assertions,
			SimulatedAt: row.SimulatedAt, Scenario: row.Scenario, Now: s.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("save workflow simulation: %w", err)
		}
		one := []models.WorkflowSimulation{out}
		current(testedNow(wf), one)
		out = one[0]
		return nil
	})
	return out, err
}

/*
Delete removes a simulation.
*/
func (s *Simulations) Delete(ctx context.Context, m authz.Membership, ref SimulationRef) error {
	if err := requireManage(m); err != nil {
		return err
	}
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findSimulation(ctx, q, m.Workspace.ID, ref); err != nil {
			return err
		}
		return q.DeleteWorkflowSimulation(ctx, models.DeleteWorkflowSimulationParams{
			WorkspaceID: m.Workspace.ID, WorkflowID: ref.Workflow, ID: ref.Simulation,
		})
	})
}

/*
RunInput is which simulations to run, every one when none is named,
against which copy, and the stream their progress goes out on.
*/
type RunInput struct {
	Only     []ids.UUID
	Version  string
	StreamID string
}

/*
Run plays a workflow's simulations against one copy of it, keeping how
each went on that copy.
*/
func (s *Simulations) Run(ctx context.Context, m authz.Membership, workflowID ids.UUID, in RunInput,
) ([]models.WorkflowSimulation, error) {
	if err := requireManage(m); err != nil {
		return nil, err
	}
	if s.Engine == nil {
		return nil, ErrSimulatorOff
	}
	if in.Version != VersionLive {
		in.Version = VersionDraft
	}
	cases, err := s.List(ctx, m, workflowID)
	if err != nil {
		return nil, err
	}
	if len(in.Only) > 0 {
		cases = slices.DeleteFunc(cases, func(c models.WorkflowSimulation) bool {
			return !slices.Contains(in.Only, c.ID)
		})
	}
	if len(cases) > maxSimulationsAtOnce {
		return nil, ErrSimulationInvalid.With("reason", "Run at most 20 simulations at once.")
	}
	copied, err := s.copyOf(ctx, m, workflowID, in.Version)
	if err != nil {
		return nil, err
	}

	out := make([]models.WorkflowSimulation, 0, len(cases))
	for _, c := range cases {
		res := s.play(ctx, playing{m: m, c: c, copy: copied, version: in.Version, stream: in.StreamID})
		kept, err := s.keep(ctx, m, c, keptResult{version: in.Version, result: res, tested: copied.tested})
		if err != nil {
			return nil, err
		}
		out = append(out, kept)
	}
	return out, s.fresh(ctx, m, workflowID, out)
}

/*
copied is the copy of a workflow a run plays: its definition and what
a run on it tests.
*/
type copied struct {
	spec   []byte
	tested string
}

/*
copyOf reads the definition of the workflow's draft or live version.
*/
func (s *Simulations) copyOf(ctx context.Context, m authz.Membership, workflowID ids.UUID, version string,
) (copied, error) {
	var out copied
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := findWorkflow(ctx, q, m.Workspace.ID, workflowID)
		if err != nil {
			return err
		}
		t := testedNow(wf)
		if version != VersionLive {
			out = copied{spec: wf.Draft, tested: t.draft}
			return nil
		}
		if wf.LiveVersionID == nil {
			return ErrWorkflowNotLive
		}
		v, err := q.FindWorkflowVersion(ctx, models.FindWorkflowVersionParams{
			WorkspaceID: m.Workspace.ID, ID: *wf.LiveVersionID,
		})
		if err != nil {
			return fmt.Errorf("find workflow version: %w", err)
		}
		out = copied{spec: v.Spec, tested: t.live}
		return nil
	})
	return out, err
}

/*
asCase is a saved simulation in the shape the shared builder checks.
*/
func asCase(c models.WorkflowSimulation) models.ProcedureTestCase {
	return models.ProcedureTestCase{
		ID: c.ID, WorkspaceID: c.WorkspaceID, Name: c.Name, SourceConversationID: c.SourceConversationID,
		Persona: c.Persona, Messages: c.Messages, ActionMocks: c.ActionMocks, Assertions: c.Assertions,
		SimulatedAt: c.SimulatedAt, Scenario: c.Scenario, CreatedByMemberID: c.CreatedByMemberID,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

/*
build applies a simulation's changes to its current shape, checking
them as an agent's are; checks only workflows have are taken out of
the shared check and checked here.
*/
func build(current models.ProcedureTestCase, in procedures.TestCaseInput) (models.ProcedureTestCase, error) {
	checks := in.Assertions
	in.Assertions = shared(checks)
	if checks != nil && in.Assertions == nil {
		in.Assertions = checks[:0:0]
	}
	row, err := procedures.BuildTestCase(current, in)
	if err != nil {
		return row, err
	}
	if checks != nil {
		row.Assertions = marshal(checks)
	}
	return row, nil
}

/*
findWorkflow returns one of the workspace's workflows.
*/
func findWorkflow(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Workflow, error) {
	wf, err := q.FindWorkflow(ctx, models.FindWorkflowParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return wf, ErrWorkflowNotFound
	}
	if err != nil {
		return wf, fmt.Errorf("find workflow: %w", err)
	}
	return wf, nil
}

/*
findSimulation returns one saved simulation of one of the workspace's
workflows.
*/
func findSimulation(ctx context.Context, q *models.Queries, ws ids.UUID, ref SimulationRef,
) (models.WorkflowSimulation, error) {
	if _, err := findWorkflow(ctx, q, ws, ref.Workflow); err != nil {
		return models.WorkflowSimulation{}, err
	}
	out, err := q.FindWorkflowSimulation(ctx, models.FindWorkflowSimulationParams{
		WorkspaceID: ws, WorkflowID: ref.Workflow, ID: ref.Simulation,
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrSimulationNotFound
	}
	if err != nil {
		return out, fmt.Errorf("find workflow simulation: %w", err)
	}
	return out, nil
}

/*
newSimulation is the workspace and workflow a simulation is saved for,
and the conversation it was made from, if any.
*/
type newSimulation struct {
	ws       ids.UUID
	workflow ids.UUID
	source   *ids.UUID
}

/*
roomFor refuses a simulation past a workflow's limit, or one made from
a conversation that does not exist.
*/
func roomFor(ctx context.Context, q *models.Queries, in newSimulation) error {
	ws, workflowID, source := in.ws, in.workflow, in.source
	if _, err := findWorkflow(ctx, q, ws, workflowID); err != nil {
		return err
	}
	existing, err := q.ListWorkflowSimulations(ctx, models.ListWorkflowSimulationsParams{
		WorkspaceID: ws, WorkflowID: workflowID,
	})
	if err != nil {
		return fmt.Errorf("count workflow simulations: %w", err)
	}
	if len(existing) >= maxSimulations {
		return ErrSimulationInvalid.With("reason", "A workflow keeps at most 100 simulations.")
	}
	if source == nil {
		return nil
	}
	if _, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: *source}); err != nil {
		return ErrSimulationInvalid.With("reason", "The conversation does not exist.").OnField("source_conversation_id")
	}
	return nil
}

/*
clipName fits a name in the 120 characters a simulation's name may
have.
*/
func clipName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) <= 120 {
		return string(runes)
	}
	return string(runes[:117]) + "..."
}

/*
memberOf is the teammate behind a membership, if a person.
*/
func memberOf(m authz.Membership) *ids.UUID {
	if m.Member.ID == (ids.UUID{}) {
		return nil
	}
	id := m.Member.ID
	return &id
}
