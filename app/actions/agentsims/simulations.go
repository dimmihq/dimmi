/*
|--------------------------------------------------------------------------
| Agent Simulations
|--------------------------------------------------------------------------
|
| A teammate tries an agent the way a customer would before anyone else
| does: a preview turn answers as the agent's draft, or its live
| version, and nothing reaches a customer or the team's systems. Saved
| simulations are chats the agent must keep getting right; they run on
| the engine procedures use, against the draft or the live version, and
| keep how each last went so the two can be compared before publishing.
|
*/

package agentsims

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Which copy of an agent a simulation answers as.
*/
const (
	VersionDraft = "draft"
	VersionLive  = "live"
)

/*
Simulation limits: the saved simulations an agent keeps and runs at
once, and the customer messages a simulation made from a conversation
keeps.
*/
const (
	maxAgentSimulations  = 100
	maxSimulationsAtOnce = 20
	maxImportedMessages  = 12
)

/*
The failures of agent simulations.
*/
var (
	ErrSimulationNotFound = fault.New(fault.NotFound, "agent_simulation_not_found",
		"This agent has no simulation with this id.")
	ErrAgentNotLive = fault.New(fault.Invalid, "agent_not_live", "This agent has no live version.").
			OnField("version")
	ErrSimulatorOff = fault.New(fault.Unavailable, "simulator_unavailable",
		"The simulator is not available: no agent can answer yet.")
)

/*
Simulations runs agents' previews and saved simulations.

Rules:
  - Reading needs ai:read; previews, running and changing simulations
    ai_agent:manage.

Errors: [ErrAgentNotFound], [ErrSimulationNotFound], [ErrAgentNotLive],
[ErrSimulatorOff], a [fault] naming the field it refuses, or a wrapped
database error.
*/
type Simulations struct {
	DB         *database.DB
	Clock      clock.Clock
	Simulator  procedures.Simulator
	Procedures *procedures.Procedures
}

/*
PreviewInput is one preview turn: the agent and which copy answers,
the conversation so far or a real one to replay, the customer's
message, the persona, the mocks by action public id, the state handed
back by the last turn, the waiting writes confirmed since, and the
stream the steps go out on.
*/
type PreviewInput struct {
	Membership         authz.Membership
	AgentID            ids.UUID
	Version            string
	History            []pipeline.SimMessage
	Message            string
	Persona            pipeline.SimPersona
	Mocks              map[string]actionkit.Mock
	State              *pipeline.SimState
	Confirmed          []string
	SourceConversation *ids.UUID
	StreamID           string
}

/*
Preview runs one turn of the agent as a customer would see it.
*/
func (s *Simulations) Preview(ctx context.Context, in PreviewInput) (*pipeline.SimulationResult, error) {
	if err := ai.RequireAgentManage(in.Membership); err != nil {
		return nil, err
	}
	if in.SourceConversation != nil && len(in.History) == 0 {
		script, err := s.Procedures.ReadConversation(ctx, in.Membership, *in.SourceConversation)
		if err != nil {
			return nil, err
		}
		in.History, in.Message = procedures.SplitReplay(script, in.Message)
	}
	if strings.TrimSpace(in.Message) == "" && len(in.Confirmed) == 0 {
		return nil, fault.New(fault.Invalid, "simulation_invalid", "Write the customer's message.").OnField("message")
	}
	turn := s.turn(in.AgentID, in.Version)
	return turn(ctx, procedures.SimulateInput{
		Membership: in.Membership, History: in.History, Message: in.Message, Persona: in.Persona,
		Mocks: in.Mocks, State: in.State, Confirmed: in.Confirmed, StreamID: in.StreamID,
	})
}

/*
copyOf is the definition a simulation answers as: the working copy, or
nil for the live version the agent's columns hold.
*/
func (s *Simulations) copyOf(ctx context.Context, m authz.Membership, agentID ids.UUID, version string,
) (*agents.Definition, error) {
	var def *agents.Definition
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		agent, err := findAgent(ctx, q, m.Workspace.ID, agentID)
		if err != nil {
			return err
		}
		if version == VersionLive {
			if agent.Status == "draft" {
				return ErrAgentNotLive
			}
			return nil
		}
		placements, err := agents.Placements(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		working, err := agents.Working(agent, placements[agent.ID])
		def = &working
		return err
	})
	return def, err
}

/*
List returns an agent's saved simulations, oldest first.
*/
func (s *Simulations) List(ctx context.Context, m authz.Membership, agentID ids.UUID,
) ([]models.AiAgentTestCase, error) {
	if err := ai.RequireAgentRead(m); err != nil {
		return nil, err
	}
	var out []models.AiAgentTestCase
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findAgent(ctx, q, m.Workspace.ID, agentID); err != nil {
			return err
		}
		var err error
		out, err = q.ListAgentTestCases(ctx, models.ListAgentTestCasesParams{
			WorkspaceID: m.Workspace.ID, AgentID: agentID,
		})
		return err
	})
	return out, err
}

/*
SimulationRef names one saved simulation of an agent.
*/
type SimulationRef struct {
	Agent      ids.UUID
	Simulation ids.UUID
}

/*
Show returns one saved simulation.
*/
func (s *Simulations) Show(ctx context.Context, m authz.Membership, ref SimulationRef,
) (models.AiAgentTestCase, error) {
	if err := ai.RequireAgentRead(m); err != nil {
		return models.AiAgentTestCase{}, err
	}
	var out models.AiAgentTestCase
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = findSimulation(ctx, models.New(tx), m.Workspace.ID, ref)
		return err
	})
	return out, err
}

/*
Create saves a simulation of an agent.
*/
func (s *Simulations) Create(ctx context.Context, m authz.Membership, agentID ids.UUID,
	in procedures.TestCaseInput,
) (models.AiAgentTestCase, error) {
	if err := ai.RequireAgentManage(m); err != nil {
		return models.AiAgentTestCase{}, err
	}
	if in.Name == nil {
		return models.AiAgentTestCase{}, procedures.ErrTestCaseInvalid.With("reason", "Name the simulation.").
			OnField("name")
	}
	var out models.AiAgentTestCase
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		room := newSimulation{agent: agentID, source: in.SourceConversation}
		if err := roomForSimulation(ctx, q, m.Workspace.ID, room); err != nil {
			return err
		}
		row, err := procedures.BuildTestCase(models.ProcedureTestCase{}, in)
		if err != nil {
			return err
		}
		out, err = q.CreateAgentTestCase(ctx, models.CreateAgentTestCaseParams{
			WorkspaceID: m.Workspace.ID, AgentID: agentID, Name: row.Name,
			SourceConversationID: in.SourceConversation, Persona: row.Persona, Messages: row.Messages,
			ActionMocks: row.ActionMocks, Assertions: row.Assertions, SimulatedAt: row.SimulatedAt,
			MemberID: memberOf(m), Now: s.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("save agent simulation: %w", err)
		}
		return nil
	})
	return out, err
}

/*
FromConversation saves a real conversation as a simulation: its
customer messages, in order, become the script.
*/
func (s *Simulations) FromConversation(ctx context.Context, m authz.Membership, agentID, conversation ids.UUID,
) (models.AiAgentTestCase, error) {
	if err := ai.RequireAgentManage(m); err != nil {
		return models.AiAgentTestCase{}, err
	}
	script, err := s.Procedures.ReadConversation(ctx, m, conversation)
	if err != nil {
		return models.AiAgentTestCase{}, err
	}
	var messages []procedures.TestMessage
	for _, msg := range script {
		if msg.From == "customer" && len(messages) < maxImportedMessages {
			messages = append(messages, procedures.TestMessage{Text: msg.Text})
		}
	}
	if len(messages) == 0 {
		return models.AiAgentTestCase{}, procedures.ErrTestCaseInvalid.
			With("reason", "The conversation has no customer message to replay.").OnField("source_conversation_id")
	}
	name := clipName("From a conversation: " + messages[0].Text)
	return s.Create(ctx, m, agentID, procedures.TestCaseInput{
		Name: &name, SourceConversation: &conversation, Messages: messages,
	})
}

/*
Update saves a changed simulation.
*/
func (s *Simulations) Update(ctx context.Context, m authz.Membership, ref SimulationRef,
	in procedures.TestCaseInput,
) (models.AiAgentTestCase, error) {
	if err := ai.RequireAgentManage(m); err != nil {
		return models.AiAgentTestCase{}, err
	}
	var out models.AiAgentTestCase
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findSimulation(ctx, q, m.Workspace.ID, ref)
		if err != nil {
			return err
		}
		row, err := procedures.BuildTestCase(asProcedureCase(current), in)
		if err != nil {
			return err
		}
		out, err = q.UpdateAgentTestCase(ctx, models.UpdateAgentTestCaseParams{
			WorkspaceID: m.Workspace.ID, AgentID: ref.Agent, ID: ref.Simulation, Name: row.Name,
			Persona: row.Persona, Messages: row.Messages, ActionMocks: row.ActionMocks, Assertions: row.Assertions,
			SimulatedAt: row.SimulatedAt, Now: s.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("save agent simulation: %w", err)
		}
		return nil
	})
	return out, err
}

/*
Delete removes a simulation.
*/
func (s *Simulations) Delete(ctx context.Context, m authz.Membership, ref SimulationRef) error {
	if err := ai.RequireAgentManage(m); err != nil {
		return err
	}
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findSimulation(ctx, q, m.Workspace.ID, ref); err != nil {
			return err
		}
		return q.DeleteAgentTestCase(ctx, models.DeleteAgentTestCaseParams{
			WorkspaceID: m.Workspace.ID, AgentID: ref.Agent, ID: ref.Simulation,
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
Run plays an agent's simulations against one copy of it, keeping how
each went on that copy.
*/
func (s *Simulations) Run(ctx context.Context, m authz.Membership, agentID ids.UUID, in RunInput,
) ([]models.AiAgentTestCase, error) {
	if err := ai.RequireAgentManage(m); err != nil {
		return nil, err
	}
	if s.Simulator == nil || s.Procedures == nil {
		return nil, ErrSimulatorOff
	}
	cases, err := s.List(ctx, m, agentID)
	if err != nil {
		return nil, err
	}
	if len(in.Only) > 0 {
		cases = slices.DeleteFunc(cases, func(c models.AiAgentTestCase) bool { return !slices.Contains(in.Only, c.ID) })
	}
	if len(cases) > maxSimulationsAtOnce {
		return nil, procedures.ErrTestCaseInvalid.With("reason", "Run at most 20 simulations at once.")
	}
	if _, err := s.copyOf(ctx, m, agentID, in.Version); err != nil {
		return nil, err
	}

	out := make([]models.AiAgentTestCase, 0, len(cases))
	for _, c := range cases {
		res := s.Procedures.PlayCase(ctx, procedures.CaseRun{
			Membership: m, Case: asProcedureCase(c), PublicID: models.PublicID(models.PrefixAiAgentTestCase, c.ID),
			Run: procedures.TestRun{StreamID: in.StreamID}, Turn: s.turn(agentID, in.Version),
		})
		kept, err := s.keep(ctx, m, c, keptResult{version: in.Version, result: res})
		if err != nil {
			return nil, err
		}
		out = append(out, kept)
	}
	return out, nil
}

/*
keptResult is how one run went, on which copy.
*/
type keptResult struct {
	version string
	result  procedures.TestResult
}

/*
keep saves how a simulation's run went on the copy it ran against.
*/
func (s *Simulations) keep(ctx context.Context, m authz.Membership, c models.AiAgentTestCase, k keptResult,
) (models.AiAgentTestCase, error) {
	raw, _ := json.Marshal(k.result)
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if k.version == VersionLive {
			return q.SetAgentTestCaseLiveResult(ctx, models.SetAgentTestCaseLiveResultParams{
				Result: raw, WorkspaceID: m.Workspace.ID, ID: c.ID,
			})
		}
		return q.SetAgentTestCaseDraftResult(ctx, models.SetAgentTestCaseDraftResultParams{
			Result: raw, WorkspaceID: m.Workspace.ID, ID: c.ID,
		})
	})
	if err != nil {
		return c, fmt.Errorf("keep simulation result: %w", err)
	}
	if k.version == VersionLive {
		c.LiveResult = raw
	} else {
		c.DraftResult = raw
	}
	return c, nil
}

/*
asProcedureCase is a saved simulation in the shape the shared engine
plays.
*/
func asProcedureCase(c models.AiAgentTestCase) models.ProcedureTestCase {
	return models.ProcedureTestCase{
		ID: c.ID, WorkspaceID: c.WorkspaceID, Name: c.Name, SourceConversationID: c.SourceConversationID,
		Persona: c.Persona, Messages: c.Messages, ActionMocks: c.ActionMocks, Assertions: c.Assertions,
		SimulatedAt: c.SimulatedAt, CreatedByMemberID: c.CreatedByMemberID, CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

/*
findSimulation returns one saved simulation of one of the workspace's
agents.
*/
func findSimulation(ctx context.Context, q *models.Queries, ws ids.UUID, ref SimulationRef,
) (models.AiAgentTestCase, error) {
	if _, err := findAgent(ctx, q, ws, ref.Agent); err != nil {
		return models.AiAgentTestCase{}, err
	}
	out, err := q.FindAgentTestCase(ctx, models.FindAgentTestCaseParams{
		WorkspaceID: ws, AgentID: ref.Agent, ID: ref.Simulation,
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrSimulationNotFound
	}
	if err != nil {
		return out, fmt.Errorf("find agent simulation: %w", err)
	}
	return out, nil
}

/*
newSimulation is the agent a simulation is saved for and the
conversation it was made from, if any.
*/
type newSimulation struct {
	agent  ids.UUID
	source *ids.UUID
}

/*
roomForSimulation refuses a simulation past an agent's limit, or one
made from a conversation that does not exist.
*/
func roomForSimulation(ctx context.Context, q *models.Queries, ws ids.UUID, in newSimulation) error {
	agentID, source := in.agent, in.source
	if _, err := findAgent(ctx, q, ws, agentID); err != nil {
		return err
	}
	existing, err := q.ListAgentTestCases(ctx, models.ListAgentTestCasesParams{WorkspaceID: ws, AgentID: agentID})
	if err != nil {
		return fmt.Errorf("count agent simulations: %w", err)
	}
	if len(existing) >= maxAgentSimulations {
		return procedures.ErrTestCaseInvalid.With("reason", "An agent keeps at most 100 simulations.")
	}
	if source == nil {
		return nil
	}
	if _, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: *source}); err != nil {
		return procedures.ErrTestCaseInvalid.With("reason", "The conversation does not exist.").
			OnField("source_conversation_id")
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
findAgent returns one of the workspace's agents.
*/
func findAgent(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.AiAgent, error) {
	agent, err := q.FindAgentByID(ctx, models.FindAgentByIDParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return agent, ai.ErrAgentNotFound
	}
	if err != nil {
		return agent, fmt.Errorf("find agent: %w", err)
	}
	return agent, nil
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
