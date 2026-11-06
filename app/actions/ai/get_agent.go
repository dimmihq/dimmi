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
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/app/support/prompts"
	"github.com/dimmihq/dimmi/config"
)

/*
GetAgentInput is who reads the agent.
*/
type GetAgentInput struct {
	Membership authz.Membership
}

/*
AgentView is the agent with everything the settings page needs: its
connections, the providers it could connect, the model catalog,
whether it could go live and the prompt version its answers use. It
holds rows, so it is shaped by a resource before it leaves the server.
*/
type AgentView struct {
	Agent         models.AiAgent
	Keys          []models.AiProviderKey
	Providers     []ProviderDescriptor
	Catalog       map[string][]CatalogEntry
	Ready         bool
	ReadinessErrs []string
	Managed       bool
	PromptVersion string
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
	Prices *aiprices.Table
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
		agent, managedKey, err := a.provision(ctx, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}

		keys, err := models.New(tx).ListProviderKeys(ctx, in.Membership.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list provider keys: %w", err)
		}

		view = &AgentView{
			Agent:         agent,
			Keys:          keys,
			Providers:     providers(a.Config),
			Catalog:       catalog(a.Prices),
			Managed:       managedKey != nil,
			PromptVersion: promptVersion(agent, a.Config),
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
provision makes sure the agent and, when the install offers one, the
built-in connection exist, pointing a fresh agent's answer role at the
built-in defaults.
*/
func (a *GetAiAgent) provision(ctx context.Context, tx *database.Tx,
	workspaceID ids.UUID,
) (models.AiAgent, *models.AiProviderKey, error) {
	deps := seedDeps{clock: a.Clock, config: a.Config}

	agent, err := ensureAgent(ctx, deps, tx, workspaceID)
	if err != nil {
		return agent, nil, err
	}

	managedKey, err := ensureManagedKey(ctx, deps, tx, workspaceID)
	if err != nil {
		return agent, nil, err
	}
	if managedKey == nil || !defaultRoles(&agent, managedKey) {
		return agent, managedKey, nil
	}

	agent, err = models.New(tx).UpdateAgent(ctx, updateParams(&agent, a.Clock.Now()))
	if err != nil {
		return agent, nil, fmt.Errorf("save default roles: %w", err)
	}
	return agent, managedKey, nil
}

/*
seedDeps are the two things the provisioning helpers both need.
*/
type seedDeps struct {
	clock  clock.Clock
	config config.AIConfig
}

/*
ensureAgent returns the workspace's agent, creating it at defaults
when missing.
*/
func ensureAgent(ctx context.Context, d seedDeps, tx *database.Tx, workspaceID ids.UUID) (models.AiAgent, error) {
	agent, err := models.New(tx).FindAgent(ctx, workspaceID)
	if err == nil {
		return agent, nil
	}

	capMicros := int64(d.config.DefaultMonthlyCapUSD * 1_000_000)
	agent, err = models.New(tx).CreateAgent(ctx, models.CreateAgentParams{
		WorkspaceID: workspaceID, MonthlySpendCapMicros: &capMicros, Now: d.clock.Now(),
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
func ensureManagedKey(ctx context.Context, d seedDeps, tx *database.Tx,
	workspaceID ids.UUID,
) (*models.AiProviderKey, error) {
	if !managedConfigured(d.config) {
		return nil, nil
	}

	key, err := models.New(tx).FindManagedProviderKey(ctx, workspaceID)
	if err == nil {
		return &key, nil
	}

	modelsJSON, _ := json.Marshal(managedModelIDs())
	created, err := models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: workspaceID, Name: "Dimmi (built-in)", Provider: "dimmi",
		AuthType: "managed", Status: "valid", StatusReason: new("built-in provider"),
		Models: modelsJSON, ProviderInfo: json.RawMessage("{}"), Config: json.RawMessage("{}"),
		Source: "system", Now: d.clock.Now(),
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
	if agent.AnswerKeyID == nil {
		answer := "claude-sonnet-5-5"
		agent.AnswerKeyID = &key.ID
		agent.AnswerModel = &answer
		return true
	}
	return false
}

/*
providers lists the connectable providers, marking which the install
can actually serve.
*/
func providers(cfg config.AIConfig) []ProviderDescriptor {
	managed := cfg.Managed
	managedUp := managed.Anthropic.APIKey != "" || managed.OpenAI.APIKey != "" || managed.OpenWeight.APIKey != ""

	return []ProviderDescriptor{
		{
			ID: enums.AIProviderDimmi, Name: "Dimmi (built-in)",
			AuthType: "managed", Available: managedUp,
		},
		{
			ID: enums.AIProviderAnthropic, Name: "Anthropic",
			AuthType: "api_key", KeyRequired: true, Available: true,
			DocsURL: "https://console.anthropic.com/settings/keys",
		},
		{
			ID: enums.AIProviderOpenAI, Name: "OpenAI",
			AuthType: "api_key", KeyRequired: true, Available: true,
			DocsURL: "https://platform.openai.com/api-keys",
		},
		{
			ID: enums.AIProviderChatGPT, Name: "ChatGPT subscription",
			AuthType: "oauth", Available: cfg.ChatGPT.Enabled,
		},
		{
			ID: enums.AIProviderEngy, Name: "Engy (open weight)",
			AuthType: "api_key", KeyRequired: true, Available: true, DocsURL: "https://engy.ai",
		},
		{
			ID: enums.AIProviderOpenAICompatible, Name: "OpenAI-compatible",
			AuthType: "api_key", Available: true,
		},
	}
}

/*
catalog lists the model pickers' entries per provider.
*/
func catalog(prices *aiprices.Table) map[string][]CatalogEntry {
	out := map[string][]CatalogEntry{}
	for _, e := range aimodels.Managed() {
		out[string(enums.AIProviderDimmi)] = append(out[string(enums.AIProviderDimmi)], catalogEntry(e, prices))
	}
	for _, e := range aimodels.ChatGPT() {
		out[string(enums.AIProviderChatGPT)] = append(out[string(enums.AIProviderChatGPT)], catalogEntry(e, prices))
	}
	return out
}

/*
readiness answers whether the agent could go live and why not: it
needs an answer model on a working connection that offers it, and at
least one channel. Classifying a question is not a model role the
workspace configures.
*/
func readiness(agent models.AiAgent, keys []models.AiProviderKey) (bool, []string) {
	var errs []string

	if !answerUsable(agent, keys) {
		errs = append(errs, "answer model not configured")
	}
	if len(agent.EnabledChannels) == 0 {
		errs = append(errs, "no channels enabled")
	}

	return len(errs) == 0, errs
}

/*
answerUsable reports whether the answer role points at a model its
connection can serve: the built-in connection always can, any other
must have validated and, when it listed its models, list this one.
*/
func answerUsable(agent models.AiAgent, keys []models.AiProviderKey) bool {
	if agent.AnswerKeyID == nil || agent.AnswerModel == nil || *agent.AnswerModel == "" {
		return false
	}

	for _, key := range keys {
		if key.ID != *agent.AnswerKeyID {
			continue
		}
		if key.Provider == string(enums.AIProviderDimmi) {
			return true
		}
		return key.Status == "valid" && offers(key, *agent.AnswerModel)
	}

	return false
}

/*
offers reports whether the connection lists the model, or lists no
models at all and so cannot be held to a list.
*/
func offers(key models.AiProviderKey, model string) bool {
	var listed []string
	if err := json.Unmarshal(key.Models, &listed); err != nil || len(listed) == 0 {
		return true
	}

	return slices.Contains(listed, model)
}

/*
promptVersion is the prompt set the agent's answers use: its own pin
when that version is compiled in, the install's otherwise.
*/
func promptVersion(agent models.AiAgent, cfg config.AIConfig) string {
	loader, err := prompts.NewLoader(cfg.Prompts.Version, nil)
	if err != nil {
		return cfg.Prompts.Version
	}
	if agent.PromptVersion != nil {
		return loader.Version(*agent.PromptVersion)
	}
	return loader.Version("")
}
