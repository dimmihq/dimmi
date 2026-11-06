/*
|--------------------------------------------------------------------------
| Update Provider Key
|--------------------------------------------------------------------------
|
| A connection's mutable bits: its name, endpoint and config, and
| optionally a rotated secret. Any credential change bumps
| updated_at, which retires the cached driver on its own.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateKeyInput is a key's editable fields; empty keeps stored values.
*/
type UpdateKeyInput struct {
	Membership authz.Membership
	KeyID      ids.UUID
	Name       string
	BaseURL    *string
	APIKey     *string
	Config     map[string]any
}

/*
UpdateProviderKey saves a connection's fields.

Rules:
  - The caller needs ai:keys:write.
  - A new secret seals before it saves and revalidates.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type UpdateProviderKey struct {
	DB    *database.DB
	Clock clock.Clock
	Keys  *encryption.Keys
	LLM   *managers.LLMManager
}

/*
Handle saves and returns the key.
*/
func (a *UpdateProviderKey) Handle(ctx context.Context, in UpdateKeyInput) (*models.AiProviderKey, error) {
	if err := RequireAgentKeys(in.Membership); err != nil {
		return nil, err
	}

	var key models.AiProviderKey

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		out, err := a.update(ctx, tx, in)
		key = out
		return err
	})
	if err != nil {
		return nil, err
	}

	return &key, nil
}

/*
update applies the patch, seals a rotated credential and revalidates
the key it changed, inside the key's transaction.
*/
func (a *UpdateProviderKey) update(ctx context.Context, tx *database.Tx,
	in UpdateKeyInput,
) (models.AiProviderKey, error) {
	key, err := findKey(ctx, tx, in.Membership.Workspace.ID, in.KeyID)
	if err != nil {
		return key, err
	}

	if in.Name != "" {
		key.Name = in.Name
	}
	if in.BaseURL != nil {
		if err := a.checkBaseURL(*in.BaseURL); err != nil {
			return key, err
		}
		key.BaseUrl = in.BaseURL
	}
	if in.Config != nil {
		key.Config, _ = json.Marshal(in.Config)
	}

	rotated := in.APIKey != nil && *in.APIKey != ""
	if rotated {
		sealed, err := managers.SealCredentials(ctx, a.Keys, key, managers.Credentials{APIKey: *in.APIKey})
		if err != nil {
			return key, err
		}
		key.CredentialsEncrypted = sealed
		key.KeyHint = nullable(keyHint(*in.APIKey))
	}

	key, err = models.New(tx).UpdateProviderKey(ctx, models.UpdateProviderKeyParams{
		WorkspaceID: key.WorkspaceID, ID: key.ID, Name: key.Name, BaseUrl: key.BaseUrl,
		Region: key.Region, AuthType: key.AuthType, CredentialsEncrypted: key.CredentialsEncrypted,
		KeyHint: key.KeyHint, Config: key.Config, Now: a.Clock.Now(),
	})
	if err != nil {
		return key, fmt.Errorf("update provider key: %w", err)
	}

	if rotated {
		key, err = keyValidator{llm: a.LLM, clock: a.Clock}.recordValidation(ctx, tx, key)
		if err != nil {
			return key, fmt.Errorf("update key status: %w", err)
		}
	}

	return key, nil
}

/*
checkBaseURL refuses an endpoint a credential may not be sent to.
*/
func (a *UpdateProviderKey) checkBaseURL(raw string) error {
	if raw == "" || a.LLM == nil {
		return nil
	}
	return a.LLM.CheckBaseURL(raw)
}

/*
ListKeysInput is who asks.
*/
type ListKeysInput struct {
	Membership authz.Membership
}

/*
ListProviderKeys returns the workspace's connections.

Rules:
  - The caller needs ai:keys:read or ai:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListProviderKeys struct {
	DB *database.DB
}

/*
Handle lists the keys.
*/
func (a *ListProviderKeys) Handle(ctx context.Context, in ListKeysInput) ([]models.AiProviderKey, error) {
	if !in.Membership.Can("ai:keys:read") && !in.Membership.Can("ai:read") {
		return nil, exceptions.ErrForbidden
	}

	var keys []models.AiProviderKey
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		keys, err = models.New(tx).ListProviderKeys(ctx, in.Membership.Workspace.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list provider keys: %w", err)
	}
	return keys, nil
}
