/*
|--------------------------------------------------------------------------
| Knowledge Settings
|--------------------------------------------------------------------------
|
| Each workspace binds one embedding model. A new workspace starts
| building the install's built-in embedder when there is one, else the
| first OpenAI key it has, else it searches by keywords alone. A member
| may pick another model: it is checked with one tiny embedding, then
| built in the background while searches stay on the old model, and
| switched in only once it covers everything. Picking a model whose
| vectors are still kept switches back at once.
|
*/

package knowledge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Defaults for a workspace's own OpenAI key.
*/
const (
	defaultOpenAIModel = "text-embedding-3-small"
	defaultDims        = 1024
	validateTimeout    = 15 * time.Second
)

/*
errModelInvalid is a model the workspace cannot embed with.
*/
func errModelInvalid(message string) error {
	return fault.New(fault.Invalid, "embedding_model_invalid", message).OnField("model")
}

/*
SettingsView is the workspace's binding with what the settings page
shows beside it: the built-in model, "" when the install has none.
*/
type SettingsView struct {
	Settings     models.KnowledgeSetting
	BuiltinModel string
}

/*
SettingsInput is a member's choice of model: a provider key, nil for
the built-in embedder, a model and its size, each defaulted when empty.
*/
type SettingsInput struct {
	EmbeddingKeyID *ids.UUID
	Model          string
	Dims           int32
}

/*
EnsureKnowledgeSettings gives every new workspace its default binding.
*/
type EnsureKnowledgeSettings struct {
	Vectors *Vectors
}

/*
Name names the initializer.
*/
func (e *EnsureKnowledgeSettings) Name() string { return "knowledge_settings" }

/*
Initialize binds the default model unless the workspace has a binding.
*/
func (e *EnsureKnowledgeSettings) Initialize(ctx context.Context, tx *database.Tx, w models.Workspace) error {
	_, err := e.Vectors.ensure(ctx, tx, w.ID)
	return err
}

/*
ensure returns the workspace's binding, creating the default one and
queueing its build when it has none.
*/
func (v *Vectors) ensure(ctx context.Context, tx *database.Tx, ws ids.UUID) (models.KnowledgeSetting, error) {
	q := models.New(tx)
	s, found, err := settings(ctx, q, ws)
	if err != nil || found {
		return s, err
	}
	model, keyID, err := v.defaultBinding(ctx, q, ws)
	if err != nil {
		return s, err
	}
	now := v.Clock.Now()
	params := models.InsertKnowledgeSettingsParams{WorkspaceID: ws, EmbeddingsStatus: "none", Now: now}
	if model != "" {
		params.PendingModelKey, params.PendingKeyID, params.EmbeddingsStatus = &model, keyID, "building"
		params.StartedAt = &now
	}
	if err := q.InsertKnowledgeSettings(ctx, params); err != nil {
		return s, fmt.Errorf("create knowledge settings: %w", err)
	}
	if model != "" {
		args := jobs.ReembedWorkspaceKnowledgeArgs{WorkspaceID: ws, ModelKey: model}
		if err := v.Queue.PushTx(ctx, tx, args); err != nil {
			return s, fmt.Errorf("queue knowledge build: %w", err)
		}
	}
	s, _, err = settings(ctx, q, ws)
	return s, err
}

/*
defaultBinding is the model a workspace starts with: the built-in
embedder, else its first OpenAI key, else none.
*/
func (v *Vectors) defaultBinding(ctx context.Context, q *models.Queries, ws ids.UUID) (string, *ids.UUID, error) {
	if v.Embeddings.Builtin() {
		return v.Embeddings.BuiltinModelKey(), nil, nil
	}
	keys, err := q.ListProviderKeys(ctx, ws)
	if err != nil {
		return "", nil, fmt.Errorf("list provider keys: %w", err)
	}
	for _, k := range keys {
		if k.Provider == string(enums.AIProviderOpenAI) && k.Status != "invalid" {
			key := ModelKey{Provider: k.Provider, Model: defaultOpenAIModel, Dims: defaultDims}
			return key.String(), &k.ID, nil
		}
	}
	return "", nil, nil
}

/*
Settings serves a workspace's knowledge settings to its members.
*/
type Settings struct {
	Vectors *Vectors
}

/*
Get returns the workspace's binding, creating the default one.

Rules:
  - The caller needs ai:read.
*/
func (a *Settings) Get(ctx context.Context, m authz.Membership) (SettingsView, error) {
	if !m.Can("ai:read") {
		return SettingsView{}, exceptions.ErrForbidden
	}
	var view SettingsView
	ws := m.Workspace.ID
	err := a.Vectors.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, _ *models.Queries) error {
		var err error
		view.Settings, err = a.Vectors.ensure(ctx, tx, ws)
		return err
	})
	view.BuiltinModel = a.Vectors.Embeddings.BuiltinModelKey()
	return view, err
}

/*
Update picks the model the workspace embeds with, after one tiny test
embedding, and starts building it.

Rules:
  - The caller needs ai:write.
  - The key must be a live key of the workspace whose provider embeds.

Errors: [exceptions.ErrForbidden], embedding_model_invalid.
*/
func (a *Settings) Update(ctx context.Context, m authz.Membership, in SettingsInput) (SettingsView, error) {
	if !m.Can("ai:write") {
		return SettingsView{}, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	b, err := a.resolve(ctx, ws, in)
	if err != nil {
		return SettingsView{}, err
	}
	model := b.model.String()
	err = a.Vectors.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		if _, err := a.Vectors.ensure(ctx, tx, ws); err != nil {
			return err
		}
		if _, err := q.SetPendingEmbeddingModel(ctx, models.SetPendingEmbeddingModelParams{
			PendingModelKey: &model, PendingKeyID: b.keyID, Now: a.Vectors.Clock.Now(), WorkspaceID: ws,
		}); err != nil {
			return fmt.Errorf("pick embedding model: %w", err)
		}
		return a.Vectors.Queue.PushTx(ctx, tx, jobs.ReembedWorkspaceKnowledgeArgs{WorkspaceID: ws, ModelKey: model})
	})
	if err != nil {
		return SettingsView{}, err
	}
	return a.Get(ctx, m)
}

/*
resolve turns the choice into a binding and checks it with one tiny
embedding at the chosen size.
*/
func (a *Settings) resolve(ctx context.Context, ws ids.UUID, in SettingsInput) (binding, error) {
	b, err := a.choice(ctx, ws, in)
	if err != nil {
		return b, err
	}
	var drv contracts.Embeddings
	err = a.Vectors.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		drv, err = a.Vectors.driverFor(ctx, q, ws, b)
		return err
	})
	if err != nil {
		return b, errModelInvalid("This embedder cannot be reached.")
	}
	check, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()
	resp, err := drv.Embed(check, contracts.EmbedRequest{
		Model: b.model.Model, Inputs: []string{"dimmi"}, Dimensions: int(b.model.Dims),
		InputType: contracts.EmbedInputDocument,
	})
	if err != nil || len(resp.Vectors) != 1 || len(resp.Vectors[0]) != int(b.model.Dims) {
		return b, errModelInvalid(fmt.Sprintf("%s could not embed a test text at %d dimensions.",
			b.model.Model, b.model.Dims))
	}
	return b, nil
}

/*
choice fills the defaults of the choice and checks its key.
*/
func (a *Settings) choice(ctx context.Context, ws ids.UUID, in SettingsInput) (binding, error) {
	dims := int32(defaultDims)
	if in.Dims > 0 {
		dims = in.Dims
	}
	if in.EmbeddingKeyID == nil {
		builtin, err := ParseModelKey(a.Vectors.Embeddings.BuiltinModelKey())
		if err != nil {
			return binding{}, errModelInvalid("This install has no built-in embedder; pick a provider key.")
		}
		if in.Dims > 0 {
			builtin.Dims = in.Dims
		}
		return binding{model: builtin}, nil
	}
	var key models.AiProviderKey
	err := a.Vectors.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		key, err = q.FindProviderKey(ctx, models.FindProviderKeyParams{WorkspaceID: ws, ID: *in.EmbeddingKeyID})
		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return binding{}, errModelInvalid("No provider key of this workspace has this id.")
	}
	if err != nil {
		return binding{}, fmt.Errorf("find embedding key: %w", err)
	}
	if !a.Vectors.Embeddings.Embeds(key.Provider) {
		return binding{}, errModelInvalid("This provider offers no embedding models.")
	}
	model := in.Model
	if model == "" && key.Provider == string(enums.AIProviderOpenAI) {
		model = defaultOpenAIModel
	}
	if model == "" {
		return binding{}, errModelInvalid("Name the embedding model this server runs.")
	}
	return binding{model: ModelKey{Provider: key.Provider, Model: model, Dims: dims}, keyID: &key.ID}, nil
}

/*
KeyInUse reports whether the workspace embeds through the key, as the
active or the pending model's.
*/
func (v *Vectors) KeyInUse(ctx context.Context, ws, keyID ids.UUID) (bool, error) {
	var used bool
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		s, found, err := settings(ctx, q, ws)
		used = found && (sameID(s.EmbeddingKeyID, keyID) || sameID(s.PendingKeyID, keyID))
		return err
	})
	return used, err
}

/*
sameID reports whether a nullable id is the id.
*/
func sameID(a *ids.UUID, b ids.UUID) bool {
	return a != nil && *a == b
}
