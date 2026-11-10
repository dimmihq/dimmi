/*
|--------------------------------------------------------------------------
| Knowledge Settings Controller
|--------------------------------------------------------------------------
|
| GET /knowledge/settings shows the embedding model the workspace's
| searches use, the one being built and how far it got. PATCH picks
| another model; searches stay on the active one until the new one
| covers everything.
|
*/

package v1

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
KnowledgeReembed is the progress of the model being built: distinct
chunk inputs in all, embedded, and refused by the provider.
*/
type KnowledgeReembed struct {
	Total     int32      `json:"total"`
	Done      int32      `json:"done"`
	Failed    int32      `json:"failed"`
	StartedAt *time.Time `json:"started_at"`
}

/*
KnowledgeSettings is the workspace's embedding binding as the API
shows it. Model keys read "<provider>:<model>@<dims>"; a null key id
is the built-in embedder.
*/
type KnowledgeSettings struct {
	Object           string           `json:"object"`
	ActiveModel      *string          `json:"active_model"`
	EmbeddingKeyID   *string          `json:"embedding_key_id"`
	PendingModel     *string          `json:"pending_model"`
	PendingKeyID     *string          `json:"pending_key_id"`
	PreviousModel    *string          `json:"previous_model"`
	EmbeddingsStatus string           `json:"embeddings_status"`
	EmbeddingsError  *string          `json:"embeddings_error"`
	Reembed          KnowledgeReembed `json:"reembed"`
	SwitchedAt       *time.Time       `json:"switched_at"`
	BuiltinModel     *string          `json:"builtin_model"`
}

/*
KnowledgeSettingsOutput is the settings.
*/
type KnowledgeSettingsOutput struct {
	Body fhttp.Resource[KnowledgeSettings]
}

/*
KnowledgeSettingsController serves the knowledge settings.
*/
type KnowledgeSettingsController struct {
	Settings *knowledge.Settings
	IDs      *ids.Registry
}

/*
Show returns the settings.
*/
func (c *KnowledgeSettingsController) Show(ctx context.Context, _ *requests.KnowledgeSettingsRequest,
) (*KnowledgeSettingsOutput, error) {
	view, err := c.Settings.Get(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	return c.output(view), nil
}

/*
Update picks the model and returns the settings.
*/
func (c *KnowledgeSettingsController) Update(ctx context.Context, in *requests.UpdateKnowledgeSettingsRequest,
) (*KnowledgeSettingsOutput, error) {
	input := knowledge.SettingsInput{Model: in.Body.Model, Dims: in.Body.Dims}
	if key := in.Body.EmbeddingKeyID; key != nil && *key != "" {
		id, err := c.IDs.Decode(string(models.PrefixAiProviderKey), *key)
		if err != nil {
			return nil, fault.New(fault.Invalid, "invalid_id", "the id is not valid").OnField("embedding_key_id")
		}
		input.EmbeddingKeyID = &id
	}
	view, err := c.Settings.Update(ctx, membership(ctx), input)
	if err != nil {
		return nil, err
	}
	return c.output(view), nil
}

/*
output shapes the view.
*/
func (c *KnowledgeSettingsController) output(view knowledge.SettingsView) *KnowledgeSettingsOutput {
	s := view.Settings
	out := KnowledgeSettings{
		Object: "knowledge_settings", ActiveModel: s.ActiveModelKey, PendingModel: s.PendingModelKey,
		PreviousModel: s.PreviousModelKey, EmbeddingsStatus: s.EmbeddingsStatus, EmbeddingsError: s.EmbeddingsError,
		EmbeddingKeyID: c.keyID(s.EmbeddingKeyID), PendingKeyID: c.keyID(s.PendingKeyID), SwitchedAt: s.SwitchedAt,
		Reembed: KnowledgeReembed{
			Total: s.ReembedTotal, Done: s.ReembedDone, Failed: s.ReembedFailed, StartedAt: s.ReembedStartedAt,
		},
	}
	if view.BuiltinModel != "" {
		out.BuiltinModel = &view.BuiltinModel
	}
	return &KnowledgeSettingsOutput{Body: fhttp.NewResource(out)}
}

/*
keyID is a key's public id, nil for none.
*/
func (c *KnowledgeSettingsController) keyID(id *ids.UUID) *string {
	if id == nil {
		return nil
	}
	public := ids.Encode(string(models.PrefixAiProviderKey), *id)
	return &public
}
