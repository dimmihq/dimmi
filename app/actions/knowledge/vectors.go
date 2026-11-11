/*
|--------------------------------------------------------------------------
| Knowledge Vectors
|--------------------------------------------------------------------------
|
| The workspace's knowledge settings bind the one model every query is
| embedded with, so a search never mixes vector spaces, whatever keys
| come and go. A query vector is cached per workspace, model and query,
| so a repeated question costs nothing, and never shared across
| workspaces. The embedder is reached only through the bound key, or
| the install's built-in embedder when none is bound; a bound key that
| is gone leaves search on keywords until a member picks another.
|
*/

package knowledge

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/dimmi"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/config"
)

/*
Reasons a search answered without vectors.
*/
var (
	ErrNoEmbeddings  = errors.New("knowledge: no embedding model bound")
	ErrKeyRemoved    = errors.New("knowledge: embedding key removed")
	ErrEmbedTimeout  = errors.New("knowledge: query embedding timed out")
	ErrEmbedFailed   = errors.New("knowledge: query embedding failed")
	ErrVectorTimeout = errors.New("knowledge: vector search timed out")
)

/*
queryVectorTTL is how long a query's vector is kept.
*/
const queryVectorTTL = 24 * time.Hour

/*
Vectors is the workspace's vector index: it binds models, embeds
content and queries, and searches by meaning. DB holds the vectors,
Embeddings builds drivers, Cache keeps query vectors, Queue runs the
embedding jobs, Config bounds the waits and Prices prices what the
embeddings cost.
*/
type Vectors struct {
	DB         *database.DB
	Embeddings *managers.EmbeddingsManager
	Cache      *cache.Store
	Clock      clock.Clock
	Queue      *queue.Queue
	Config     config.AIConfig
	Prices     *aiprices.Table
}

/*
queryVector is a query's vector and how it was obtained.
*/
type queryVector struct {
	vector []float32
	model  string
	keyID  *ids.UUID
	usage  *contracts.LLMUsage
}

/*
inWorkspace runs fn in a transaction scoped to the workspace.
*/
func (v *Vectors) inWorkspace(ctx context.Context, ws ids.UUID,
	fn func(context.Context, *database.Tx, *models.Queries) error,
) error {
	return v.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, tx, models.New(tx))
	})
}

/*
settings returns the workspace's binding; found is false when it has
none yet.
*/
func settings(ctx context.Context, q *models.Queries, ws ids.UUID) (models.KnowledgeSetting, bool, error) {
	s, err := q.FindKnowledgeSettings(ctx, ws)
	if errors.Is(err, database.ErrNotFound) {
		return s, false, nil
	}
	if err != nil {
		return s, false, fmt.Errorf("find knowledge settings: %w", err)
	}
	return s, true, nil
}

/*
binding is one model a workspace embeds with and the key that reaches
it, nil for the built-in embedder.
*/
type binding struct {
	model ModelKey
	keyID *ids.UUID
}

/*
driverFor returns the driver for a binding, through its key, or the
built-in embedder for a built-in model.
*/
func (v *Vectors) driverFor(ctx context.Context, q *models.Queries, ws ids.UUID, b binding,
) (contracts.Embeddings, error) {
	if b.model.Provider == dimmi.Provider {
		return v.Embeddings.ForKey(ctx, nil)
	}
	if b.keyID == nil {
		return nil, ErrKeyRemoved
	}
	key, err := q.FindProviderKey(ctx, models.FindProviderKeyParams{WorkspaceID: ws, ID: *b.keyID})
	if errors.Is(err, database.ErrNotFound) {
		return nil, ErrKeyRemoved
	}
	if err != nil {
		return nil, fmt.Errorf("find embedding key: %w", err)
	}
	return v.Embeddings.ForKey(ctx, &key)
}

/*
queryVector embeds the query with the workspace's active model, from
the cache when it was asked before, within the timeout.
*/
func (v *Vectors) queryVector(ctx context.Context, ws ids.UUID, query string, timeout time.Duration,
) (queryVector, error) {
	var out queryVector
	var drv contracts.Embeddings
	var model ModelKey
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		s, found, err := settings(ctx, q, ws)
		if err != nil {
			return err
		}
		if !found || s.ActiveModelKey == nil {
			return ErrNoEmbeddings
		}
		if model, err = ParseModelKey(*s.ActiveModelKey); err != nil {
			return ErrNoEmbeddings
		}
		out.model, out.keyID = *s.ActiveModelKey, s.EmbeddingKeyID
		drv, err = v.driverFor(ctx, q, ws, binding{model: model, keyID: s.EmbeddingKeyID})
		return err
	})
	if err != nil {
		return out, err
	}
	cacheKey := queryCacheKey(ws, out.model, query)
	if cached, err := v.Cache.Get(ctx, cacheKey); err == nil && len(cached) == 4*int(model.Dims) {
		out.vector, out.keyID = decodeVector(cached), nil
		return out, nil
	}
	embedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := drv.Embed(embedCtx, contracts.EmbedRequest{
		Model: model.Model, Inputs: []string{query}, Dimensions: int(model.Dims),
		InputType: contracts.EmbedInputQuery,
	})
	if err != nil {
		if embedCtx.Err() != nil && ctx.Err() == nil {
			return out, ErrEmbedTimeout
		}
		return out, fmt.Errorf("%w: %w", ErrEmbedFailed, err)
	}
	out.vector = resp.Vectors[0]
	out.usage = &contracts.LLMUsage{InputTokens: resp.Usage.InputTokens, Reported: resp.Usage.Reported}
	_ = v.Cache.Set(context.WithoutCancel(ctx), cacheKey, encodeVector(out.vector), queryVectorTTL)
	return out, nil
}

/*
queryTimeout is the caller's wait for a query vector, or the
configured default.
*/
func (v *Vectors) queryTimeout(asked time.Duration) time.Duration {
	if asked > 0 {
		return asked
	}
	return time.Duration(max(v.Config.Knowledge.QueryEmbedTimeoutMS, 1)) * time.Millisecond
}

/*
vectorTimeout bounds the vector query itself.
*/
func (v *Vectors) vectorTimeout() time.Duration {
	return time.Duration(max(v.Config.Knowledge.VectorTimeoutMS, 1)) * time.Millisecond
}

/*
queryCacheKey names a query vector: workspace, model and the query
normalized for case and spacing, hashed.
*/
func queryCacheKey(ws ids.UUID, model, query string) string {
	normal := strings.Join(strings.Fields(strings.ToLower(query)), " ")
	return "knowledge:query_vector:" + ws.String() + ":" + model + ":" + hashing.SHA256Hex([]byte(normal))
}

/*
encodeVector writes a vector as little-endian float32s.
*/
func encodeVector(v []float32) []byte {
	out := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(x))
	}
	return out
}

/*
decodeVector reads a vector encodeVector wrote.
*/
func decodeVector(b []byte) []float32 {
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out
}

/*
ActiveModel is the model the workspace's queries are embedded with,
"" when none is bound.
*/
func (v *Vectors) ActiveModel(ctx context.Context, ws ids.UUID) (string, error) {
	var model string
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		s, found, err := settings(ctx, q, ws)
		if found && s.ActiveModelKey != nil {
			model = *s.ActiveModelKey
		}
		return err
	})
	return model, err
}
