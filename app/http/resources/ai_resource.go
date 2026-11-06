/*
|--------------------------------------------------------------------------
| AI Resources
|--------------------------------------------------------------------------
|
| The agent and its connections as the dashboard sees them: every
| setting a teammate can change, and never a credential, a workspace
| or member id, or a provider's raw error. A key tells its status, a
| safe reason and its last four; every id is a public one.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIProviderKey is one connection, secrets withheld. Models lists the
model ids the provider offered when the key last validated.
*/
type AIProviderKey struct {
	ID              string     `json:"id"`
	Object          string     `json:"object"`
	Name            string     `json:"name"`
	Provider        string     `json:"provider"`
	BaseURL         *string    `json:"base_url"`
	Region          *string    `json:"region"`
	AuthType        string     `json:"auth_type"`
	KeyHint         *string    `json:"key_hint"`
	Status          string     `json:"status"`
	StatusReason    *string    `json:"status_reason"`
	Models          []string   `json:"models"`
	ModelsCount     int        `json:"models_count"`
	Source          string     `json:"source"`
	LastValidatedAt *time.Time `json:"last_validated_at"`
	LastUsedAt      *time.Time `json:"last_used_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

/*
NewAIProviderKey shapes a key for the API.
*/
func NewAIProviderKey(k models.AiProviderKey) AIProviderKey {
	listed := []string{}
	_ = json.Unmarshal(k.Models, &listed)
	if listed == nil {
		listed = []string{}
	}

	return AIProviderKey{
		ID: models.PublicID(models.PrefixAiProviderKey, k.ID), Object: "ai_provider_key",
		Name: k.Name, Provider: k.Provider, BaseURL: k.BaseUrl, Region: k.Region,
		AuthType: k.AuthType, KeyHint: k.KeyHint, Status: k.Status, StatusReason: k.StatusReason,
		Models: listed, ModelsCount: len(listed), Source: k.Source,
		LastValidatedAt: k.LastValidatedAt, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt, UpdatedAt: k.UpdatedAt,
	}
}

/*
NewAIProviderKeys shapes a list of keys for the API.
*/
func NewAIProviderKeys(keys []models.AiProviderKey) []AIProviderKey {
	out := make([]AIProviderKey, len(keys))
	for i, k := range keys {
		out[i] = NewAIProviderKey(k)
	}
	return out
}

/*
AIAgent is the agent's editable shape. Connection, team and tag ids are
public ids; MonthlySpendCapMicros is null when there is no cap.
HandoffMode is full or triage_only; AlwaysHandoffTagIDs are the tags
that always send a conversation to the team.
*/
type AIAgent struct {
	ID                    string          `json:"id"`
	Object                string          `json:"object"`
	Name                  string          `json:"name"`
	Enabled               bool            `json:"enabled"`
	ProductName           *string         `json:"product_name"`
	Tone                  string          `json:"tone"`
	Guidance              string          `json:"guidance"`
	EnabledChannels       []string        `json:"enabled_channels"`
	Schedule              string          `json:"schedule"`
	OfficeHours           json.RawMessage `json:"office_hours"`
	AssignedBehavior      string          `json:"assigned_behavior"`
	AnswerKeyID           *string         `json:"answer_key_id"`
	AnswerModel           *string         `json:"answer_model"`
	FallbackKeyID         *string         `json:"fallback_key_id"`
	FallbackModel         *string         `json:"fallback_model"`
	MaxOutputTokens       int             `json:"max_output_tokens"`
	MaxAiReplies          int             `json:"max_ai_replies"`
	UnknownBehavior       string          `json:"unknown_behavior"`
	GroundingMode         string          `json:"grounding_mode"`
	RequireCitations      bool            `json:"require_citations"`
	Languages             []string        `json:"languages"`
	DefaultLanguage       string          `json:"default_language"`
	Streaming             bool            `json:"streaming"`
	IncludeContactContext string          `json:"include_contact_context"`
	PiiRedaction          bool            `json:"pii_redaction"`
	LinkAllowlist         []string        `json:"link_allowlist"`
	HandoffTeamID         *string         `json:"handoff_team_id"`
	RaisePriority         bool            `json:"raise_priority_on_handoff"`
	HandoffMode           string          `json:"handoff_mode"`
	AlwaysHandoffTagIDs   []string        `json:"always_handoff_tag_ids"`
	MonthlySpendCapMicros *int64          `json:"monthly_spend_cap_micros"`
	ActionsEnabled        bool            `json:"actions_enabled"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

/*
NewAIAgent shapes the agent for the API.
*/
func NewAIAgent(a models.AiAgent) AIAgent {
	handoff := ai.ReadHandoff(a.Handoff)

	return AIAgent{
		ID: models.PublicID(models.PrefixAiAgent, a.ID), Object: "ai_agent",
		Name: a.Name, Enabled: a.Enabled, ProductName: a.ProductName,
		Tone: a.Tone, Guidance: a.Guidance, EnabledChannels: orEmpty(a.EnabledChannels),
		Schedule: a.Schedule, OfficeHours: a.OfficeHours, AssignedBehavior: a.AssignedBehavior,
		AnswerKeyID: publicPtr(models.PrefixAiProviderKey, a.AnswerKeyID), AnswerModel: a.AnswerModel,
		FallbackKeyID: publicPtr(models.PrefixAiProviderKey, a.FallbackKeyID), FallbackModel: a.FallbackModel,
		MaxOutputTokens: int(a.MaxOutputTokens), MaxAiReplies: int(a.MaxAiReplies),
		UnknownBehavior: a.UnknownBehavior, GroundingMode: a.GroundingMode, RequireCitations: a.RequireCitations,
		Languages: orEmpty(a.Languages), DefaultLanguage: a.DefaultLanguage, Streaming: a.Streaming,
		IncludeContactContext: a.IncludeContactContext, PiiRedaction: a.PiiRedaction,
		LinkAllowlist: orEmpty(a.LinkAllowlist),
		HandoffTeamID: publicPtr(models.PrefixTeam, handoff.TargetTeamID), RaisePriority: handoff.RaisePriority,
		HandoffMode: string(handoff.Mode), AlwaysHandoffTagIDs: publicIDs(models.PrefixTag, handoff.AlwaysTags),
		MonthlySpendCapMicros: a.MonthlySpendCapMicros, ActionsEnabled: a.ActionsEnabled, UpdatedAt: a.UpdatedAt,
	}
}

/*
AIAgentView is everything the agent's settings page reads in one call:
the agent, its connections, the providers it could connect, the model
catalog, whether it could go live and why not, and the prompt version
its answers use.
*/
type AIAgentView struct {
	Agent           AIAgent                      `json:"agent"`
	Keys            []AIProviderKey              `json:"keys"`
	Providers       []ai.ProviderDescriptor      `json:"providers"`
	Catalog         map[string][]ai.CatalogEntry `json:"catalog"`
	Ready           bool                         `json:"ready"`
	ReadinessErrors []string                     `json:"readiness_errors"`
	Managed         bool                         `json:"managed"`
	PromptVersion   string                       `json:"prompt_version"`
}

/*
AIChatGPTConnection is where a subscription sign-in stands: pending,
with how long to wait before asking again, or connected, with the new
connection's public id and the plan account.
*/
type AIChatGPTConnection struct {
	Status          string `json:"status"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
	KeyID           string `json:"key_id,omitempty"`
	AccountID       string `json:"account_id,omitempty"`
}

/*
NewAIChatGPTConnection shapes a sign-in's progress for the API.
*/
func NewAIChatGPTConnection(c ai.CompleteAnswer) AIChatGPTConnection {
	out := AIChatGPTConnection{Status: c.Status, IntervalSeconds: c.IntervalSeconds, AccountID: c.AccountID}
	if c.Status == ai.ConnectStatusConnected {
		out.KeyID = models.PublicID(models.PrefixAiProviderKey, c.KeyID)
	}
	return out
}

/*
NewAIAgentView shapes the settings view for the API.
*/
func NewAIAgentView(v ai.AgentView) AIAgentView {
	return AIAgentView{
		Agent: NewAIAgent(v.Agent), Keys: NewAIProviderKeys(v.Keys), Providers: v.Providers, Catalog: v.Catalog,
		Ready: v.Ready, ReadinessErrors: orEmpty(v.ReadinessErrs), Managed: v.Managed, PromptVersion: v.PromptVersion,
	}
}
