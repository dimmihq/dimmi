/*
|--------------------------------------------------------------------------
| AI Agent Resources
|--------------------------------------------------------------------------
|
| An agent as the API reads it: what customers get, its working copy
| under draft, whether the two differ, and where it stands. Every id in
| and out is a public one; credentials never leave the server.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIAgent is one agent. Its fields are what customers get: the published
definition. Draft is the working copy, the same as the published one
when HasUnpublishedChanges is false. Status is draft (never published),
live or paused; IsDefault marks the agent that answers every
conversation no other agent takes. LiveVersion is the published
version's number, 0 before the first publish.
*/
type AIAgent struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	AIAgentDefinition
	IsDefault             bool              `json:"is_default"`
	Role                  string            `json:"role"`
	Template              *string           `json:"template"`
	Status                string            `json:"status"`
	Enabled               bool              `json:"enabled"`
	LockVersion           int32             `json:"lock_version"`
	HasUnpublishedChanges bool              `json:"has_unpublished_changes"`
	LiveVersion           int32             `json:"live_version"`
	MaxOutputTokens       int               `json:"max_output_tokens"`
	GroundingMode         string            `json:"grounding_mode"`
	ActionsEnabled        bool              `json:"actions_enabled"`
	Draft                 AIAgentDefinition `json:"draft"`
	PublishedAt           *time.Time        `json:"published_at"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

/*
NewAIAgent shapes an agent for the API, from its row, its working copy
and where it answers, with its live version's number.
*/
func NewAIAgent(s ai.AgentSummary, liveVersion int32) AIAgent {
	a := s.Agent
	return AIAgent{
		ID: models.PublicID(models.PrefixAiAgent, a.ID), Object: "ai_agent",
		AIAgentDefinition: NewAIAgentDefinition(agents.Published(a, s.Placement)),
		IsDefault:         a.IsDefault, Role: a.Role, Template: a.TemplateKey, Status: a.Status, Enabled: a.Enabled,
		LockVersion: a.LockVersion, HasUnpublishedChanges: s.Unpublished, LiveVersion: liveVersion,
		MaxOutputTokens: int(a.MaxOutputTokens), GroundingMode: a.GroundingMode, ActionsEnabled: a.ActionsEnabled,
		Draft: NewAIAgentDefinition(s.Working), PublishedAt: a.PublishedAt, CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

/*
NewAIAgentSummary shapes an agent of the list for the API.
*/
func NewAIAgentSummary(s ai.AgentSummary) AIAgent {
	return NewAIAgent(s, 0)
}

/*
AIAgentView is everything Agent Studio reads in one call: the agent,
the workspace's shared AI settings, the connections and providers, the
model catalog, whether the working copy could go live and why not, and
the prompt version its answers use.
*/
type AIAgentView struct {
	Agent           AIAgent                      `json:"agent"`
	Settings        AISettings                   `json:"settings"`
	Keys            []AIProviderKey              `json:"keys"`
	Providers       []ai.ProviderDescriptor      `json:"providers"`
	Catalog         map[string][]ai.CatalogEntry `json:"catalog"`
	Ready           bool                         `json:"ready"`
	ReadinessErrors []string                     `json:"readiness_errors"`
	Managed         bool                         `json:"managed"`
	PromptVersion   string                       `json:"prompt_version"`
}

/*
NewAIAgentView shapes the studio view for the API.
*/
func NewAIAgentView(v ai.AgentView) AIAgentView {
	return AIAgentView{
		Agent: NewAIAgent(ai.AgentSummary{
			Agent: v.Agent, Working: v.Working, Placement: v.Placement, Unpublished: v.Unpublished,
		}, v.LiveVersion),
		Settings: NewAISettings(v.Settings), Keys: NewAIProviderKeys(v.Keys), Providers: v.Providers,
		Catalog: v.Catalog, Ready: v.Ready, ReadinessErrors: orEmpty(v.ReadinessErrs), Managed: v.Managed,
		PromptVersion: v.PromptVersion,
	}
}
