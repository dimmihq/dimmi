/*
|--------------------------------------------------------------------------
| Agents
|--------------------------------------------------------------------------
|
| A workspace runs many agents. One is the default: it answers every
| conversation no other agent takes, exists from the workspace's first
| read, and cannot be deleted. Every other agent starts as a draft from
| a template or from nothing. An agent is edited as a draft and only
| reaches customers when it is published, the way procedures are.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/config"
)

/*
ErrAgentNotFound says the workspace has no such agent.
*/
var ErrAgentNotFound = fault.New(fault.NotFound, "agent_not_found", "the agent does not exist")

/*
ErrDefaultAgent refuses to delete the default agent.
*/
var ErrDefaultAgent = fault.New(fault.Conflict, "default_agent",
	"the default agent answers every conversation no other agent takes, so it cannot be archived")

/*
ErrAgentChanged says someone else saved the agent since the editor
read it.
*/
var ErrAgentChanged = fault.New(fault.Conflict, "agent_changed", "someone else changed the agent; reload it")

/*
Agents edits a workspace's agents.

Rules:
  - Reading needs ai:read, every change ai_agent:manage.
  - Names are unique in the workspace, ignoring case.

Errors: [exceptions.ErrForbidden], [ErrAgentNotFound], a [fault]
naming the field it refuses, or a wrapped database error.
*/
type Agents struct {
	DB     *database.DB
	Clock  clock.Clock
	Config config.AIConfig
}

/*
AgentView is one agent with what the studio needs: the row customers
get, the working copy, where it answers, the workspace's shared
settings, the connections and models it may pick, and whether it could
go live.
*/
type AgentView struct {
	Agent         models.AiAgent
	Working       agents.Definition
	Placement     agents.Placement
	Unpublished   bool
	LiveVersion   int32
	Settings      models.WorkspaceAiSetting
	Keys          []models.AiProviderKey
	Providers     []ProviderDescriptor
	Catalog       map[string][]CatalogEntry
	Ready         bool
	ReadinessErrs []string
	Managed       bool
	PromptVersion string
	Ties          []Tie
}

/*
AgentSummary is one agent as the list shows it.
*/
type AgentSummary struct {
	Agent       models.AiAgent
	Working     agents.Definition
	Placement   agents.Placement
	Unpublished bool
}

/*
CreateAgentInput is who makes an agent, what it is called and the
template it starts from; an empty template starts from nothing.
*/
type CreateAgentInput struct {
	Membership  authz.Membership
	Name        string
	TemplateKey string
}

/*
Provision makes sure the workspace's AI exists: its shared settings,
its default agent and, when the install offers one, the built-in
connection the workspace answers with until it connects its own.
*/
func (a *Agents) Provision(ctx context.Context, tx *database.Tx, workspaceID ids.UUID,
) (models.AiAgent, *models.AiProviderKey, error) {
	return provision(ctx, seedDeps{clock: a.Clock, config: a.Config}, tx, workspaceID)
}

func provision(ctx context.Context, d seedDeps, tx *database.Tx,
	workspaceID ids.UUID,
) (models.AiAgent, *models.AiProviderKey, error) {
	q := models.New(tx)
	settings, err := agents.Settings(ctx, q, workspaceID, agents.Defaults{
		CapUSD: d.config.DefaultMonthlyCapUSD, Now: d.clock.Now(),
	})
	if err != nil {
		return models.AiAgent{}, nil, err
	}

	agent, err := agents.Default(ctx, q, workspaceID)
	if errors.Is(err, agents.ErrNoAgent) {
		if _, err := q.CreateDefaultAgent(ctx, models.CreateDefaultAgentParams{
			WorkspaceID: workspaceID, Now: d.clock.Now(),
		}); err != nil && !errors.Is(err, database.ErrNotFound) {
			return agent, nil, fmt.Errorf("create default agent: %w", err)
		}
		agent, err = agents.Default(ctx, q, workspaceID)
	}
	if err != nil {
		return agent, nil, err
	}

	key, err := ensureManagedKey(ctx, d, tx, workspaceID)
	if err != nil {
		return agent, nil, err
	}

	answer := aimodels.DefaultAnswer(managedFamily(d.config))
	if key == nil || settings.AnswerKeyID != nil || answer == "" {
		return agent, key, nil
	}
	settings.AnswerKeyID, settings.AnswerModel = &key.ID, &answer
	if _, err := q.UpdateWorkspaceAISettings(ctx, settingsParams(settings, d.clock)); err != nil {
		return agent, key, fmt.Errorf("save default answer model: %w", err)
	}
	return agent, key, nil
}

/*
List returns the workspace's agents, the default first.
*/
func (a *Agents) List(ctx context.Context, m authz.Membership) ([]AgentSummary, error) {
	if err := RequireAgentRead(m); err != nil {
		return nil, err
	}
	var out []AgentSummary
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, _, err := a.Provision(ctx, tx, m.Workspace.ID); err != nil {
			return err
		}
		q := models.New(tx)
		rows, err := q.ListAgents(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list agents: %w", err)
		}
		placements, err := agents.Placements(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			working, err := agents.Working(row, placements[row.ID])
			if err != nil {
				return err
			}
			out = append(out, AgentSummary{
				Agent: row, Working: working, Placement: placements[row.ID], Unpublished: unpublished(row),
			})
		}
		return nil
	})
	return out, err
}

/*
Show returns one agent ready to edit.
*/
func (a *Agents) Show(ctx context.Context, m authz.Membership, id ids.UUID) (*AgentView, error) {
	if err := RequireAgentRead(m); err != nil {
		return nil, err
	}
	var view *AgentView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, key, err := a.Provision(ctx, tx, m.Workspace.ID)
		if err != nil {
			return err
		}
		agent, err := findAgent(ctx, models.New(tx), m.Workspace.ID, id)
		if err != nil {
			return err
		}
		view, err = a.view(ctx, tx, agent, key != nil)
		return err
	})
	return view, err
}

/*
view builds an agent's view in the caller's transaction.
*/
func (a *Agents) view(ctx context.Context, tx *database.Tx, agent models.AiAgent, managed bool) (*AgentView, error) {
	q := models.New(tx)
	placements, err := agents.Placements(ctx, q, agent.WorkspaceID)
	if err != nil {
		return nil, err
	}
	working, err := agents.Working(agent, placements[agent.ID])
	if err != nil {
		return nil, err
	}
	settings, err := agents.Shared(ctx, q, agent.WorkspaceID)
	if err != nil {
		return nil, err
	}
	keys, err := q.ListProviderKeys(ctx, agent.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list provider keys: %w", err)
	}
	view := &AgentView{
		Agent: agent, Working: working, Placement: placements[agent.ID], Unpublished: unpublished(agent),
		Settings: settings, Keys: keys, Providers: providers(a.Config), Catalog: catalog(a.Config),
		Managed: managed, PromptVersion: promptVersion(agent, a.Config),
	}
	if agent.LiveVersionID != nil {
		if v, err := q.FindAgentVersionByID(ctx, models.FindAgentVersionByIDParams{
			WorkspaceID: agent.WorkspaceID, ID: *agent.LiveVersionID,
		}); err == nil {
			view.LiveVersion = v.Version
		}
	}
	view.Ready, view.ReadinessErrs = readiness(draftAgent(agent, working, settings), keys)
	if view.Ties, err = agentTies(ctx, q, agent.WorkspaceID, agent.ID); err != nil {
		return nil, err
	}
	return view, nil
}

/*
Create makes a new agent as a draft from a template.
*/
func (a *Agents) Create(ctx context.Context, in CreateAgentInput) (*AgentView, error) {
	if err := RequireAgentManage(in.Membership); err != nil {
		return nil, err
	}
	tpl, ok := TemplateByKey(in.TemplateKey)
	if !ok {
		return nil, fault.New(fault.Invalid, "template_not_found", "there is no such template").OnField("template")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = tpl.AgentName
	}

	var view *AgentView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		def, key, err := a.Provision(ctx, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}
		q := models.New(tx)
		taken := models.AgentNameTakenParams{WorkspaceID: in.Membership.Workspace.ID, Name: name}
		if err := checkName(ctx, q, taken); err != nil {
			return err
		}
		start := tpl.Definition(agents.Published(blankAgent(def), agents.Placement{}))
		start.Name = name
		draft, _ := json.Marshal(start)
		member := memberOf(in.Membership)
		agent, err := q.CreateAgent(ctx, models.CreateAgentParams{
			WorkspaceID: in.Membership.Workspace.ID, Name: name, Role: tpl.Role, TemplateKey: tpl.keyOrNil(),
			Draft: draft, MemberID: member, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}
		view, err = a.view(ctx, tx, agent, key != nil)
		return err
	})
	return view, err
}

/*
Delete archives an agent that is not the default and that nothing
relies on any more: it is turned off and leaves the list, while its
conversations, runs and history stay. While a workflow, series, place
or other agent still relies on it, it is refused with [ErrAgentTied],
listing each tie.
*/
func (a *Agents) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := RequireAgentManage(m); err != nil {
		return err
	}
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		agent, err := findAgent(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}
		if agent.IsDefault {
			return ErrDefaultAgent
		}
		ties, err := agentTies(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}
		if len(ties) > 0 {
			return ErrAgentTied.With("ties", tieMeta(ties))
		}
		now := a.Clock.Now()
		return q.DeleteAgent(ctx, models.DeleteAgentParams{WorkspaceID: m.Workspace.ID, ID: id, Now: &now})
	})
}

/*
findAgent returns one of the workspace's agents.
*/
func findAgent(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.AiAgent, error) {
	agent, err := q.FindAgentByID(ctx, models.FindAgentByIDParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return agent, ErrAgentNotFound
	}
	if err != nil {
		return agent, fmt.Errorf("find agent: %w", err)
	}
	return agent, nil
}

/*
checkName refuses an empty name, one longer than 60 characters, or one
another agent has.
*/
func checkName(ctx context.Context, q *models.Queries, p models.AgentNameTakenParams) error {
	if p.Name == "" || len([]rune(p.Name)) > 60 {
		return fault.New(fault.Invalid, "name_invalid", "give the agent a name of up to 60 characters").OnField("name")
	}
	taken, err := q.AgentNameTaken(ctx, p)
	if err != nil {
		return fmt.Errorf("check agent name: %w", err)
	}
	if taken {
		return fault.New(fault.Invalid, "name_taken", "another agent has this name").OnField("name")
	}
	return nil
}

/*
unpublished reports whether the agent has changes customers do not get
yet: a draft, or never having been published.
*/
func unpublished(agent models.AiAgent) bool {
	return agent.Status == "draft" || (len(agent.Draft) > 0 && string(agent.Draft) != "null")
}

/*
draftAgent is the agent as its working copy would answer, with the
workspace's shared models folded in.
*/
func draftAgent(agent models.AiAgent, working agents.Definition, s models.WorkspaceAiSetting) models.AiAgent {
	agents.Apply(&agent, working)
	return agents.Answering(agent, s)
}

/*
blankAgent is a fresh agent's published columns: the database
defaults, which the default agent was made with, without its identity,
models, guidance or where it answers.
*/
func blankAgent(def models.AiAgent) models.AiAgent {
	blank := def
	blank.AnswerKeyID, blank.AnswerModel, blank.FallbackKeyID, blank.FallbackModel = nil, nil, nil, nil
	blank.Guidance, blank.ProductName, blank.AvatarAttachmentID = "", nil, nil
	blank.Tone, blank.EnabledChannels, blank.ExcludedChannelConnectionIds = "friendly", []string{}, []ids.UUID{}
	blank.Handoff, blank.Messages = json.RawMessage("{}"), json.RawMessage("{}")
	blank.KnowledgeScope = json.RawMessage(`{"mode":"all"}`)
	blank.ProcedureScope = json.RawMessage(`{"mode":"all"}`)
	blank.ActionScope = json.RawMessage(`{"mode":"all"}`)
	blank.TransferAgentIds = []ids.UUID{}
	blank.Languages, blank.LinkAllowlist, blank.ContactAttributeAllowlist = []string{}, []string{}, []string{}
	return blank
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

/*
Default returns the workspace's default agent ready to edit, making
it on first use.
*/
func (a *Agents) Default(ctx context.Context, m authz.Membership) (*AgentView, error) {
	if err := RequireAgentRead(m); err != nil {
		return nil, err
	}
	var view *AgentView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		agent, key, err := a.Provision(ctx, tx, m.Workspace.ID)
		if err != nil {
			return err
		}
		view, err = a.view(ctx, tx, agent, key != nil)
		return err
	})
	return view, err
}

/*
EnsureAI is the workspace initializer that gives a new workspace its
shared AI settings, its default agent and the built-in connection.
*/
type EnsureAI struct {
	Agents *Agents
}

/*
Name names the defaults it creates.
*/
func (e *EnsureAI) Name() string { return "ai_agents" }

/*
Initialize provisions the workspace's AI unless it has it already.
*/
func (e *EnsureAI) Initialize(ctx context.Context, tx *database.Tx, w models.Workspace) error {
	_, _, err := e.Agents.Provision(ctx, tx, w.ID)
	return err
}
