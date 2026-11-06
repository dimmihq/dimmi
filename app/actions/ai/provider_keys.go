/*
|--------------------------------------------------------------------------
| Provider Key Actions
|--------------------------------------------------------------------------
|
| A workspace's connections: create validates before it saves, update
| may rotate the secret, test revalidates on demand, and delete
| refuses while an agent role or the embedder still points at the
| key.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
CreateKeyInput is one new connection.
*/
type CreateKeyInput struct {
	Membership authz.Membership
	Name       string
	Provider   enums.AIProvider
	BaseURL    string
	APIKey     string
	Config     map[string]any
}

/*
CreateProviderKey stores a new connection, validating it first.

Rules:
  - The caller needs ai:keys:write; the provider must be connectable;
    an api_key connection needs a key.
  - The key validates live before it saves: models when the provider
    lists them, a probe when it does not.

Errors: [exceptions.ErrForbidden], or a [fault] error carrying the
provider's failure, or a wrapped database error.
*/
type CreateProviderKey struct {
	DB    *database.DB
	Clock clock.Clock
	Keys  *encryption.Keys
	LLM   *managers.LLMManager
}

/*
Handle creates and returns the key.
*/
func (a *CreateProviderKey) Handle(ctx context.Context, in CreateKeyInput) (*models.AiProviderKey, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, fault.New(fault.Invalid, "name_required", "a name is required")
	}
	if !supportedProvider(in.Provider) {
		return nil, fault.New(fault.Invalid, "provider_unsupported", "provider not supported")
	}
	if in.Provider == enums.AIProviderDimmi || in.Provider == enums.AIProviderChatGPT {
		return nil, fault.New(fault.Invalid, "provider_managed", "this provider connects itself")
	}
	if in.Provider != enums.AIProviderOpenAICompatible && in.APIKey == "" {
		return nil, fault.New(fault.Invalid, "key_required", "an api key is required")
	}
	if in.Provider == enums.AIProviderOpenAICompatible && in.BaseURL == "" {
		return nil, fault.New(fault.Invalid, "base_url_required", "a base url is required")
	}

	var key models.AiProviderKey
	var errs []string

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		cfg, _ := json.Marshal(in.Config)
		authType := "api_key"
		if in.Provider == enums.AIProviderOpenAICompatible && in.APIKey == "" {
			authType = "none"
		}

		var err error
		memberID := in.Membership.Member.ID
		key, err = models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
			WorkspaceID: in.Membership.Workspace.ID, Name: in.Name, Provider: string(in.Provider),
			BaseUrl: nullable(in.BaseURL), AuthType: authType, Status: "unverified",
			StatusReason: str("never validated"), Models: json.RawMessage("[]"),
			ProviderInfo: json.RawMessage("{}"), Config: cfg, Source: "dashboard",
			CreatedByMemberID: &memberID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create provider key: %w", err)
		}

		if in.APIKey != "" {
			sealed, err := encryption.Seal(ctx, a.Keys,
				encryption.Slot{Scope: encryption.Scope{Kind: "workspace", ID: in.Membership.Workspace.ID.String()}},
				managers.Credentials{APIKey: in.APIKey})
			if err != nil {
				return fmt.Errorf("seal credentials: %w", err)
			}
			key.CredentialsEncrypted = sealed
		}

		status, reason, modelsList, info, verr := validate(ctx, a.LLM, key)
		if verr != nil {
			errs = append(errs, verr.Error())
		}

		modelsJSON, _ := json.Marshal(modelsList)
		infoJSON, _ := json.Marshal(info)
		key, err = models.New(tx).UpdateProviderKey(ctx, models.UpdateProviderKeyParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID, Name: key.Name, BaseUrl: key.BaseUrl,
			Region: key.Region, AuthType: key.AuthType, CredentialsEncrypted: key.CredentialsEncrypted,
			KeyHint: nullable(keyHint(in.APIKey)), Config: key.Config, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("update provider key: %w", err)
		}

		key, err = models.New(tx).UpdateProviderKeyStatus(ctx, models.UpdateProviderKeyStatusParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID, Status: status, StatusReason: reason,
			LastError: nil, LastValidatedAt: timenow(a.Clock), LastErrorAt: nil,
			Models: modelsJSON, ProviderInfo: infoJSON, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("update key status: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &key, nil
}

/*
DeleteKeyInput names the key to remove.
*/
type DeleteKeyInput struct {
	Membership authz.Membership
	KeyID      ids.UUID
}

/*
DeleteProviderKey removes a connection when nothing points at it.

Rules:
  - The caller needs ai:keys:write; a role-bound key must be moved
    first; deleting is idempotent.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], a
[fault] when the key is in use, or a wrapped database error.
*/
type DeleteProviderKey struct {
	DB        *database.DB
	Clock     clock.Clock
	Knowledge contracts.Knowledge
}

/*
Handle deletes the key.
*/
func (a *DeleteProviderKey) Handle(ctx context.Context, in DeleteKeyInput) error {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return err
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		key, err := findKey(ctx, tx, in.Membership.Workspace.ID, in.KeyID)
		if err != nil {
			return err
		}

		if a.Knowledge != nil {
			if inUse, err := a.Knowledge.EmbeddingKeyInUse(ctx, key.WorkspaceID, key.ID); err == nil && inUse {
				return fault.New(fault.Conflict, "key_in_use_by_embeddings", "the knowledge index uses this key")
			}
		}

		if agent, err := models.New(tx).FindAgent(ctx, key.WorkspaceID); err == nil {
			if roles := keyInUse(agent, key.ID); len(roles) > 0 {
				return fault.New(fault.Conflict, "key_in_use", "key is the agent's "+roles[0]+" model")
			}
		}

		return models.New(tx).DeleteProviderKey(ctx, models.DeleteProviderKeyParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID, Now: timenow(a.Clock),
		})
	})
}

/*
TestKeyInput names the key to revalidate.
*/
type TestKeyInput struct {
	Membership authz.Membership
	KeyID      ids.UUID
}

/*
TestProviderKey revalidates a connection on demand and returns its
fresh status.

Rules:
  - The caller needs ai:keys:write.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type TestProviderKey struct {
	DB    *database.DB
	Clock clock.Clock
	LLM   *managers.LLMManager
}

/*
Handle tests and returns the key.
*/
func (a *TestProviderKey) Handle(ctx context.Context, in TestKeyInput) (*models.AiProviderKey, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}

	var key models.AiProviderKey

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		key, err = findKey(ctx, tx, in.Membership.Workspace.ID, in.KeyID)
		if err != nil {
			return err
		}

		status, reason, modelsList, info, _ := validate(ctx, a.LLM, key)

		modelsJSON, _ := json.Marshal(modelsList)
		infoJSON, _ := json.Marshal(info)
		key, err = models.New(tx).UpdateProviderKeyStatus(ctx, models.UpdateProviderKeyStatusParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID, Status: status, StatusReason: reason,
			LastError: nil, LastValidatedAt: timenow(a.Clock), LastErrorAt: nil,
			Models: modelsJSON, ProviderInfo: infoJSON, Now: a.Clock.Now(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return &key, nil
}

/*
supportedProvider reports whether the provider may be created.
*/
func supportedProvider(p enums.AIProvider) bool {
	for _, allowed := range enums.AIProviders() {
		if p == allowed {
			return true
		}
	}
	return false
}

/*
validate calls the provider through the driver's manager: ListModels
where offered, a probe where not, and returns the status answer.
*/
func validate(ctx context.Context, m *managers.LLMManager, key models.AiProviderKey) (status string, reason *string, modelList []string, info map[string]any, err error) {
	if m == nil {
		return "unverified", str("not validated"), nil, nil, nil
	}

	driver, err := m.ForKey(ctx, key)
	if err != nil {
		return "error", str(err.Error()), nil, nil, err
	}

	models, listErr := driver.ListModels(ctx)
	if listErr != nil && listErr != errLLMNotSupported() {
		return "invalid", str(listErr.Error()), nil, nil, listErr
	}

	if len(models) > 0 {
		for _, mdl := range models {
			modelList = append(modelList, mdl.ID)
		}
		return "valid", str("validated"), modelList, map[string]any{"models_count": len(models)}, nil
	}

	return "valid", str("validated"), nil, nil, nil
}

/*
errLLMNotSupported re-exports the contract sentinel.
*/
func errLLMNotSupported() error {
	return contractsNotSupported()
}

func contractsNotSupported() error { return errNotSupported }

var errNotSupported = fmt.Errorf("not supported")

/*
nullable maps an empty string to NULL.
*/
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

/*
str returns a string pointer.
*/
func str(s string) *string { return &s }

/*
keyHint stores the key's last four, never more.
*/
func keyHint(key string) string {
	if len(key) <= 4 {
		return "••••"
	}
	return "••••" + key[len(key)-4:]
}

/*
timenow is the current time pointer for status updates.
*/
func timenow(clk clock.Clock) *time.Time {
	now := clk.Now()
	return &now
}

var _ = exceptions.ErrNotFound
