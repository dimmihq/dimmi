/*
|--------------------------------------------------------------------------
| Get AI Agent
|--------------------------------------------------------------------------
|
| The workspace's agent, ready to edit: it exists by the time the
| caller sees it, the built-in connection exists beside it, and the
| readiness answer says whether it could go live today.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
GetAgentInput is who reads the agent.
*/
type GetAgentInput struct {
	Membership authz.Membership
}

/*
AgentView is the agent with everything the settings page needs:
| readiness, keys, the providers it could connect and the catalog.
*/
type AgentView struct {
	Agent         models.AiAgent
	Keys          []models.AiProviderKey
	Providers     []ProviderDescriptor
	Catalog       map[string][]CatalogEntry
	Ready         bool
	ReadinessErrs []string
	Managed       bool
}

/*
GetAiAgent returns the workspace's agent, creating it and the
built-in connection on first use.

Rules:
  - The caller needs ai:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type GetAiAgent struct {
	DB     *database.DB
	Clock  clock.Clock
	Config config.AIConfig
}

/*
Handle returns the agent view.
*/
func (a *GetAiAgent) Handle(ctx context.Context, in GetAgentInput) (*AgentView, error) {
	if err := RequireAgentRead(in.Membership); err != nil {
		return nil, err
	}

	var view *AgentView

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		agent, err := ensureAgent(ctx, a.Clock, a.Config, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}

		managedKey, err := ensureManagedKey(ctx, a.Config, a.Clock, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}
		if managedKey != nil && defaultRoles(&agent, managedKey) {
			if agent, err = models.New(tx).UpdateAgent(ctx, updateParams(&agent, a.Clock.Now())); err != nil {
				return fmt.Errorf("save default roles: %w", err)
			}
		}

		keys, err := models.New(tx).ListProviderKeys(ctx, in.Membership.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list provider keys: %w", err)
		}

		view = &AgentView{
			Agent:     agent,
			Keys:      keys,
			Providers: providers(a.Config),
			Catalog:   catalog(),
			Managed:   managedKey != nil,
		}
		view.Ready, view.ReadinessErrs = readiness(agent, keys)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return view, nil
}

/*
ensureAgent returns the workspace's agent, creating it at defaults
when missing.
*/
func ensureAgent(ctx context.Context, clk clock.Clock, cfg config.AIConfig, tx *database.Tx, workspaceID ids.UUID) (models.AiAgent, error) {
	agent, err := models.New(tx).FindAgent(ctx, workspaceID)
	if err == nil {
		return agent, nil
	}

	capMicros := int64(cfg.DefaultMonthlyCapUSD * 1_000_000)
	agent, err = models.New(tx).CreateAgent(ctx, models.CreateAgentParams{
		WorkspaceID: workspaceID, MonthlySpendCapMicros: &capMicros, Now: clk.Now(),
	})
	if err != nil {
		return models.AiAgent{}, fmt.Errorf("create ai agent: %w", err)
	}
	return agent, nil
}

/*
ensureManagedKey returns the built-in connection, creating it and
pointing the agent's roles at the default models when the install has
managed credentials to offer.
*/
func ensureManagedKey(ctx context.Context, cfg config.AIConfig, clk clock.Clock, tx *database.Tx, workspaceID ids.UUID) (*models.AiProviderKey, error) {
	if !managedConfigured(cfg) {
		return nil, nil
	}

	key, err := models.New(tx).FindManagedProviderKey(ctx, workspaceID)
	if err == nil {
		return &key, nil
	}

	modelsJSON, _ := json.Marshal(managedModelIDs())
	created, err := models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: workspaceID, Name: "Dimmi (built-in)", Provider: "dimmi",
		AuthType: "managed", Status: "valid", StatusReason: str("built-in provider"),
		Models: modelsJSON, ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"),
		Source: "system", Now: clk.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("create managed key: %w", err)
	}
	return &created, nil
}

/*
managedConfigured reports whether the install offers the built-in
provider.
*/
func managedConfigured(cfg config.AIConfig) bool {
	m := cfg.Managed
	return m.Anthropic.APIKey != "" || m.OpenAI.APIKey != "" || m.OpenWeight.APIKey != ""
}

/*
managedModelIDs lists the catalog ids the managed connection offers
for its configured families.
*/
func managedModelIDs() []string {
	var out []string
	for _, e := range aimodels.Managed() {
		out = append(out, e.ID)
	}
	return out
}

/*
defaultRoles points an unconfigured agent's roles at the managed
connection's defaults.
*/
func defaultRoles(agent *models.AiAgent, key *models.AiProviderKey) bool {
	if key == nil {
		return false
	}
	changed := false
	answer, classifier := "claude-sonnet-5-5", "claude-haiku-5-5"
	if agent.AnswerKeyID == nil {
		agent.AnswerKeyID = &key.ID
		agent.AnswerModel = &answer
		changed = true
	}
	if agent.ClassifierKeyID == nil {
		agent.ClassifierKeyID = &key.ID
		agent.ClassifierModel = &classifier
		changed = true
	}
	return changed
}

/*
providers lists the connectable providers, marking which the install
can actually serve.
*/
func providers(cfg config.AIConfig) []ProviderDescriptor {
	managed := cfg.Managed
	managedUp := managed.Anthropic.APIKey != "" || managed.OpenAI.APIKey != "" || managed.OpenWeight.APIKey != ""

	return []ProviderDescriptor{
		{ID: enums.AIProviderDimmi, Name: "Dimmi (built-in)", AuthType: "managed", KeyRequired: false, Available: managedUp},
		{ID: enums.AIProviderAnthropic, Name: "Anthropic", AuthType: "api_key", KeyRequired: true, Available: true, DocsURL: "https://console.anthropic.com/settings/keys"},
		{ID: enums.AIProviderOpenAI, Name: "OpenAI", AuthType: "api_key", KeyRequired: true, Available: true, DocsURL: "https://platform.openai.com/api-keys"},
		{ID: enums.AIProviderChatGPT, Name: "ChatGPT subscription", AuthType: "oauth", KeyRequired: false, Available: cfg.ChatGPT.Enabled},
		{ID: enums.AIProviderEngy, Name: "Engy (open weight)", AuthType: "api_key", KeyRequired: true, Available: true, DocsURL: "https://engy.ai"},
		{ID: enums.AIProviderOpenAICompatible, Name: "OpenAI-compatible", AuthType: "api_key", KeyRequired: false, Available: true},
	}
}

/*
catalog lists the model pickers' entries per provider.
*/
func catalog() map[string][]CatalogEntry {
	out := map[string][]CatalogEntry{}
	for _, e := range aimodels.Managed() {
		out[string(enums.AIProviderDimmi)] = append(out[string(enums.AIProviderDimmi)], catalogEntry(e))
	}
	for _, e := range aimodels.ChatGPT() {
		out[string(enums.AIProviderChatGPT)] = append(out[string(enums.AIProviderChatGPT)], catalogEntry(e))
	}
	return out
}

/*
readiness answers whether the agent could go live and why not.
*/
func readiness(agent models.AiAgent, keys []models.AiProviderKey) (bool, []string) {
	var errs []string

	usable := map[ids.UUID]bool{}
	for _, key := range keys {
		if key.Status == "valid" || key.Provider == "dimmi" {
			usable[key.ID] = true
		}
	}

	if agent.AnswerKeyID == nil || agent.AnswerModel == nil || !usable[*agent.AnswerKeyID] {
		errs = append(errs, "answer model not configured")
	}
	if agent.ClassifierKeyID == nil || agent.ClassifierModel == nil || !usable[*agent.ClassifierKeyID] {
		errs = append(errs, "classifier model not configured")
	}
	if len(agent.EnabledChannels) == 0 {
		errs = append(errs, "no channels enabled")
	}

	return len(errs) == 0, errs
}
