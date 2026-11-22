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
