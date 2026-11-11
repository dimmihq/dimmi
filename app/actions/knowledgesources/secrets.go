/*
|--------------------------------------------------------------------------
| Source Secrets
|--------------------------------------------------------------------------
|
| A driver's schema marks which settings are secret. Those are taken out
| of the settings before they are stored, sealed with the workspace's
| keys bound to the source's row, and never shown again: the dashboard
| only learns that a secret is set. A change that leaves a secret blank
| keeps the sealed one, so editing a source never asks for its key again.
|
*/

package knowledgesources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
schemaSecrets reads which properties a driver's schema has and which
of them it marks secret.
*/
func schemaSecrets(d sources.Description) (map[string]bool, error) {
	var shape struct {
		Properties map[string]map[string]any `json:"properties"`
	}

	if err := json.Unmarshal(d.Schema, &shape); err != nil {
		return nil, err
	}

	out := make(map[string]bool, len(shape.Properties))
	for name, prop := range shape.Properties {
		secret, _ := prop["x-secret"].(bool)
		out[name] = secret
	}

	return out, nil
}

/*
SecretFields lists the settings a driver's schema marks secret.
*/
func SecretFields(d sources.Description) []string {
	props, _ := schemaSecrets(d)

	var out []string

	for name, secret := range props {
		if secret {
			out = append(out, name)
		}
	}

	return out
}

/*
splitSecrets parts settings into what is stored plainly and the secrets
the driver's schema marks, dropping fields the schema does not know.
*/
func splitSecrets(d sources.Description, settings map[string]any) (map[string]any, map[string]string) {
	props, _ := schemaSecrets(d)
	plain, secrets := map[string]any{}, map[string]string{}

	for name, value := range settings {
		secret, known := props[name]
		if !known {
			continue
		}

		if !secret {
			plain[name] = value

			continue
		}

		if s, ok := value.(string); ok && s != "" {
			secrets[name] = s
		}
	}

	return plain, secrets
}

func secretsAAD(id ids.UUID) []byte {
	return []byte("knowledge_sources.credentials_encrypted:" + id.String())
}

/*
sealSecrets seals the secrets for the source, nil when there are none.
*/
func (e *Engine) sealSecrets(ctx context.Context, ws, id ids.UUID, secrets map[string]string,
) (encryption.Sealed, error) {
	if len(secrets) == 0 {
		return nil, nil
	}

	sealed, err := encryption.Seal(ctx, e.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: ws.String()}, AAD: secretsAAD(id),
	}, secrets)
	if err != nil {
		return nil, fmt.Errorf("seal source secrets: %w", err)
	}

	return sealed, nil
}

/*
openSecrets opens a source's sealed secrets, none when it has none.
*/
func (e *Engine) openSecrets(ctx context.Context, s models.KnowledgeSource) (map[string]string, error) {
	if len(s.CredentialsEncrypted) == 0 {
		return map[string]string{}, nil
	}

	out, err := encryption.Open[map[string]string](ctx, e.Keys, s.CredentialsEncrypted, secretsAAD(s.ID))
	if err != nil {
		return nil, fmt.Errorf("open source secrets: %w", err)
	}

	return out, nil
}
