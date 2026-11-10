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
	if !defaultRoles(&agent, managedKey, a.Config) {
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

	modelsJSON, _ := json.Marshal(managedModelIDs())

	key, err := models.New(tx).FindManagedProviderKey(ctx, workspaceID)
	if err == nil {
		return refreshManagedKey(ctx, d, tx, key)
	}

	created, err := models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: workspaceID, Name: managedKeyName, Provider: "dimmi",
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
managedKeyName is what the built-in connection is called.
*/
const managedKeyName = "dimmi"

/*
refreshManagedKey brings an existing built-in connection up to date:
the models the install offers today, which change as the supported
catalog does, and its current name. A connection already current is
returned untouched, so reading the agent writes nothing.
*/
func refreshManagedKey(ctx context.Context, d seedDeps, tx *database.Tx,
	key models.AiProviderKey,
) (*models.AiProviderKey, error) {
	modelsJSON, _ := json.Marshal(managedModelIDs())
	var err error
	if key.Name != managedKeyName {
		key, err = models.New(tx).UpdateProviderKey(ctx, models.UpdateProviderKeyParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID, Name: managedKeyName, BaseUrl: key.BaseUrl,
			Region: key.Region, AuthType: key.AuthType, CredentialsEncrypted: key.CredentialsEncrypted,
			KeyHint: key.KeyHint, Config: key.Config, Now: d.clock.Now(),
		})
		if err != nil {
			return nil, fmt.Errorf("rename managed key: %w", err)
		}
	}
	if sameModels(key.Models, modelsJSON) {
		return &key, nil
	}

	key, err = models.New(tx).UpdateProviderKeyStatus(ctx, models.UpdateProviderKeyStatusParams{
		WorkspaceID: key.WorkspaceID, ID: key.ID, Status: key.Status, StatusReason: key.StatusReason,
		LastError: key.LastError, LastValidatedAt: key.LastValidatedAt, LastErrorAt: key.LastErrorAt,
		Models: modelsJSON, ProviderInfo: key.ProviderInfo, Now: d.clock.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("refresh managed models: %w", err)
	}
	return &key, nil
}

/*
sameModels reports whether two model lists name the same ids in the
same order, however their JSON is spaced.
*/
func sameModels(stored, want json.RawMessage) bool {
	var a, b []string
	if json.Unmarshal(stored, &a) != nil || json.Unmarshal(want, &b) != nil {
		return false
	}
	return slices.Equal(a, b)
}

/*
managedConfigured reports whether the install offers the built-in
provider.
*/
func managedConfigured(cfg config.AIConfig) bool {
	return aimodels.DefaultAnswer(managedFamily(cfg)) != ""
}

/*
managedFamily reports, for the install's config, whether a model family
has a built-in upstream to answer it.
*/
func managedFamily(cfg config.AIConfig) func(enums.AIModelFamily) bool {
	m := cfg.Managed
	return func(family enums.AIModelFamily) bool {
		switch family {
		case enums.AIModelFamilyAnthropic:
			return m.Anthropic.APIKey != ""
		case enums.AIModelFamilyOpenAI:
			return m.OpenAI.APIKey != ""
		case enums.AIModelFamilyOpenWeight:
			return m.OpenWeight.APIKey != ""
		}
		return false
	}
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
defaultRoles points an unconfigured agent's answer role at the managed
connection and the cheapest model the install's upstreams can serve, so
the agent can answer without any setup.
*/
func defaultRoles(agent *models.AiAgent, key *models.AiProviderKey, cfg config.AIConfig) bool {
	answer := aimodels.DefaultAnswer(managedFamily(cfg))
	if key == nil || agent.AnswerKeyID != nil || answer == "" {
		return false
	}
	agent.AnswerKeyID = &key.ID
	agent.AnswerModel = &answer
	return true
}

/*
providers lists the connectable providers, marking which the install
can actually serve.
*/
func providers(cfg config.AIConfig) []ProviderDescriptor {
	return []ProviderDescriptor{
		{
			ID: enums.AIProviderDimmi, Name: managedKeyName,
			AuthType: "managed", Available: managedConfigured(cfg),
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
			ID: enums.AIProviderBaseten, Name: "Baseten (open weight)",
			AuthType: "api_key", KeyRequired: true, Available: true, DocsURL: "https://www.baseten.co",
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
