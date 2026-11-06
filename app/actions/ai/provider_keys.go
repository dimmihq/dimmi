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
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxProviderKeys is how many live connections one workspace may hold.
*/
const maxProviderKeys = 20

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
  - The caller needs ai:keys; the provider must be connectable;
    an api_key connection needs a key.
  - A workspace holds at most 20 connections.
  - The key validates live as it saves: its status says whether the
    provider accepted it, with a reason safe to show.

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
	if in.BaseURL != "" && a.LLM != nil {
		if err := a.LLM.CheckBaseURL(in.BaseURL); err != nil {
			return nil, err
		}
	}

	var key models.AiProviderKey

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		out, err := a.create(ctx, tx, in)
		key = out
		return err
	})
	if err != nil {
		return nil, err
	}

	return &key, nil
}

/*
create inserts the key, seals its credential, validates it and
records the outcome, in the key's transaction.
*/
func (a *CreateProviderKey) create(ctx context.Context, tx *database.Tx, in CreateKeyInput,
) (models.AiProviderKey, error) {
	if err := checkKeyLimit(ctx, tx, in.Membership.Workspace.ID); err != nil {
		return models.AiProviderKey{}, err
	}

	cfg, _ := json.Marshal(in.Config)
	authType := "api_key"
	if in.Provider == enums.AIProviderOpenAICompatible && in.APIKey == "" {
		authType = "none"
	}

	memberID := in.Membership.Member.ID
	key, err := models.New(tx).CreateProviderKey(ctx, models.CreateProviderKeyParams{
		WorkspaceID: in.Membership.Workspace.ID, Name: in.Name, Provider: string(in.Provider),
		BaseUrl: nullable(in.BaseURL), AuthType: authType, Status: "unverified",
		StatusReason: new("never validated"), Models: json.RawMessage("[]"),
		ProviderInfo: json.RawMessage("{}"), Config: cfg, Source: "dashboard",
		CreatedByMemberID: &memberID, Now: a.Clock.Now(),
	})
	if err != nil {
		return key, fmt.Errorf("create provider key: %w", err)
	}

	if in.APIKey != "" {
		sealed, err := managers.SealCredentials(ctx, a.Keys, key, managers.Credentials{APIKey: in.APIKey})
		if err != nil {
			return key, err
		}
		key.CredentialsEncrypted = sealed
	}

	key, err = models.New(tx).UpdateProviderKey(ctx, models.UpdateProviderKeyParams{
		WorkspaceID: key.WorkspaceID, ID: key.ID, Name: key.Name, BaseUrl: key.BaseUrl,
		Region: key.Region, AuthType: key.AuthType, CredentialsEncrypted: key.CredentialsEncrypted,
		KeyHint: nullable(keyHint(in.APIKey)), Config: key.Config, Now: a.Clock.Now(),
	})
	if err != nil {
		return key, fmt.Errorf("update provider key: %w", err)
	}

	key, err = keyValidator{llm: a.LLM, clock: a.Clock}.recordValidation(ctx, tx, key)
	if err != nil {
		return key, fmt.Errorf("update key status: %w", err)
	}
	return key, nil
}

/*
checkKeyLimit refuses a connection past the workspace's limit.
*/
func checkKeyLimit(ctx context.Context, tx *database.Tx, workspaceID ids.UUID) error {
	count, err := models.New(tx).CountProviderKeys(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count provider keys: %w", err)
	}
	if count >= maxProviderKeys {
		return fault.New(fault.Invalid, "provider_keys_limit", "a workspace may hold at most 20 connections")
	}
	return nil
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
  - The caller needs ai:keys; a role-bound key must be moved
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
supportedProvider reports whether the provider may be created.
*/
func supportedProvider(p enums.AIProvider) bool {
	return slices.Contains(enums.AIProviders(), p)
}

/*
keyCheck is what validate learned about the connection: its status, a
reason safe to show anyone who may read the key, the models it lists,
and the raw failure kept for operators only.
*/
type keyCheck struct {
	status    string
	reason    *string
	modelList []string
	info      map[string]any
	err       error
}

/*
validate calls the provider through the driver's manager, listing its
models where it can, and returns the status answer.
*/
func validate(ctx context.Context, m *managers.LLMManager, key models.AiProviderKey) keyCheck {
	if m == nil {
		return keyCheck{status: "unverified", reason: new("not validated")}
	}

	driver, err := m.ForKey(ctx, key)
	if err != nil {
		return keyCheck{status: failureStatus(err), reason: new(failureReason(err)), err: err}
	}

	listed, listErr := driver.ListModels(ctx)
	if listErr != nil && !errors.Is(listErr, contracts.ErrLLMNotSupported) {
		return keyCheck{status: failureStatus(listErr), reason: new(failureReason(listErr)), err: listErr}
	}

	modelList := []string{}
	for _, mdl := range listed {
		modelList = append(modelList, mdl.ID)
	}

	return keyCheck{
		status: "valid", reason: new("validated"), modelList: modelList,
		info: map[string]any{"models_count": len(listed)},
	}
}

/*
failureStatus files a provider failure: a refused key is invalid, a
provider that could not be reached is an error worth retrying later.
*/
func failureStatus(err error) string {
	var llmErr *contracts.LLMError
	if !errors.As(err, &llmErr) {
		return "error"
	}

	refused := []contracts.LLMErrorKind{contracts.LLMErrorAuth, contracts.LLMErrorPermission, contracts.LLMErrorQuota}
	if slices.Contains(refused, llmErr.Kind) {
		return "invalid"
	}
	return "error"
}

/*
failureReason words a provider failure for the settings page without
repeating the provider's raw message, which may carry endpoints or
request details nobody should see.
*/
func failureReason(err error) string {
	var llmErr *contracts.LLMError
	if !errors.As(err, &llmErr) {
		return "the connection could not be set up"
	}

	if reason, ok := keyFailureReasons()[llmErr.Kind]; ok {
		return reason
	}
	return "the provider returned an error"
}

/*
keyFailureReasons words every kind of provider failure.
*/
func keyFailureReasons() map[contracts.LLMErrorKind]string {
	return map[contracts.LLMErrorKind]string{
		contracts.LLMErrorAuth:            "the provider rejected the key",
		contracts.LLMErrorPermission:      "the key may not use this provider's models",
		contracts.LLMErrorQuota:           "the provider account is out of credit or quota",
		contracts.LLMErrorRateLimited:     "the provider is rate limiting this key",
		contracts.LLMErrorOverloaded:      "the provider is overloaded",
		contracts.LLMErrorServer:          "the provider returned an error",
		contracts.LLMErrorTimeout:         "the provider did not answer in time",
		contracts.LLMErrorInvalidRequest:  "the provider refused the request",
		contracts.LLMErrorModelNotFound:   "the provider does not offer that model",
		contracts.LLMErrorContentFiltered: "the provider filtered the request",
		contracts.LLMErrorNetwork:         "the provider could not be reached",
		contracts.LLMErrorBlockedHost:     "the address is not reachable from this server",
		contracts.LLMErrorCanceled:        "the check was canceled",
	}
}

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
keyHint stores the key's last four, never more, and nothing without a key.
*/
func keyHint(key string) string {
	if key == "" {
		return ""
	}
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

/*
keyValidator pairs the manager and clock the checks need.
*/
type keyValidator struct {
	llm   *managers.LLMManager
	clock clock.Clock
}

/*
recordValidation validates the key against its provider and stores
the status, models and info it returned. A failure keeps the models
the key listed before, and its raw message lands in last_error, which
no response carries.
*/
func (v keyValidator) recordValidation(ctx context.Context, tx *database.Tx,
	key models.AiProviderKey,
) (models.AiProviderKey, error) {
	check := validate(ctx, v.llm, key)

	modelsJSON, infoJSON := key.Models, key.ProviderInfo
	if check.status == "valid" {
		modelsJSON, _ = json.Marshal(check.modelList)
		infoJSON, _ = json.Marshal(check.info)
	}

	var lastError *string
	var lastErrorAt *time.Time
	if check.err != nil {
		lastError, lastErrorAt = new(check.err.Error()), timenow(v.clock)
	}

	return models.New(tx).UpdateProviderKeyStatus(ctx, models.UpdateProviderKeyStatusParams{
		WorkspaceID: key.WorkspaceID, ID: key.ID, Status: check.status, StatusReason: check.reason,
		LastError: lastError, LastValidatedAt: timenow(v.clock), LastErrorAt: lastErrorAt,
		Models: modelsJSON, ProviderInfo: infoJSON, Now: v.clock.Now(),
	})
}
