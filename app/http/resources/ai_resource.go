/*
|--------------------------------------------------------------------------
| AI Resources
|--------------------------------------------------------------------------
|
| The agent and its connections as the dashboard sees them: every
| setting except the one thing never shown, a credential. A key tells
| its status and its last four; a run tells its outcome and cost.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
AIProviderKey is one connection, secrets withheld.
*/
type AIProviderKey struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Provider        string          `json:"provider"`
	BaseURL         *string         `json:"base_url"`
	AuthType        string          `json:"auth_type"`
	KeyHint         *string         `json:"key_hint"`
	Status          string          `json:"status"`
	StatusReason    *string         `json:"status_reason"`
	Models          json.RawMessage `json:"models"`
	LastValidatedAt any             `json:"last_validated_at"`
	LastUsedAt      any             `json:"last_used_at"`
	CreatedAt       any             `json:"created_at"`
}

/*
NewAIProviderKey shapes a key for the API.
*/
func NewAIProviderKey(k models.AiProviderKey) AIProviderKey {
	return AIProviderKey{
		ID: k.ID.String(), Name: k.Name, Provider: k.Provider, BaseURL: k.BaseUrl,
		AuthType: k.AuthType, KeyHint: k.KeyHint, Status: k.Status, StatusReason: k.StatusReason,
		Models: k.Models, LastValidatedAt: k.LastValidatedAt, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
	}
}

/*
AIAgent is the agent's full editable shape.
*/
type AIAgent struct {
	ID                    string          `json:"id"`
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
	ClassifierKeyID       *string         `json:"classifier_key_id"`
	ClassifierModel       *string         `json:"classifier_model"`
	FallbackKeyID         *string         `json:"fallback_key_id"`
	FallbackModel         *string         `json:"fallback_model"`
	Temperature           any             `json:"temperature"`
	MaxOutputTokens       int             `json:"max_output_tokens"`
	MaxAiReplies          int             `json:"max_ai_replies"`
	UnknownBehavior       string          `json:"unknown_behavior"`
	GroundingMode         string          `json:"grounding_mode"`
	RequireCitations      bool            `json:"require_citations"`
	MinSimilarity         any             `json:"min_similarity"`
	Languages             []string        `json:"languages"`
	DefaultLanguage       string          `json:"default_language"`
	Streaming             bool            `json:"streaming"`
	IncludeContactContext string          `json:"include_contact_context"`
	PiiRedaction          bool            `json:"pii_redaction"`
	LinkAllowlist         []string        `json:"link_allowlist"`
	Handoff               json.RawMessage `json:"handoff"`
	Messages              json.RawMessage `json:"messages"`
	MonthlySpendCapMicros *int64          `json:"monthly_spend_cap_micros"`
	ActionsEnabled        bool            `json:"actions_enabled"`
}

/*
NewAIAgent shapes the agent for the API.
*/
func NewAIAgent(a models.AiAgent) AIAgent {
	return AIAgent{
		ID: a.ID.String(), Name: a.Name, Enabled: a.Enabled, ProductName: a.ProductName,
		Tone: a.Tone, Guidance: a.Guidance, EnabledChannels: a.EnabledChannels,
		Schedule: a.Schedule, OfficeHours: a.OfficeHours, AssignedBehavior: a.AssignedBehavior,
		AnswerKeyID: uuidPtr(a.AnswerKeyID), AnswerModel: a.AnswerModel,
		ClassifierKeyID: uuidPtr(a.ClassifierKeyID), ClassifierModel: a.ClassifierModel,
		FallbackKeyID: uuidPtr(a.FallbackKeyID), FallbackModel: a.FallbackModel,
		Temperature: a.Temperature, MaxOutputTokens: int(a.MaxOutputTokens), MaxAiReplies: int(a.MaxAiReplies),
		UnknownBehavior: a.UnknownBehavior, GroundingMode: a.GroundingMode,
		RequireCitations: a.RequireCitations, MinSimilarity: a.MinSimilarity,
		Languages: a.Languages, DefaultLanguage: a.DefaultLanguage, Streaming: a.Streaming,
		IncludeContactContext: a.IncludeContactContext, PiiRedaction: a.PiiRedaction,
		LinkAllowlist: a.LinkAllowlist, Handoff: a.Handoff, Messages: a.Messages,
		MonthlySpendCapMicros: a.MonthlySpendCapMicros, ActionsEnabled: a.ActionsEnabled,
	}
}

/*
uuidPtr maps a nullable uuid to a nullable string.
*/
func uuidPtr(id *ids.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}
