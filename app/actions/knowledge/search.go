/*
|--------------------------------------------------------------------------
| Vector Search
|--------------------------------------------------------------------------
|
| The meaning half of a hybrid search: the query is embedded with the
| workspace's active model and the live chunks nearest to it are
| ranked by cosine similarity. It never fails a search. With no model
| bound, a key gone, a slow or failing provider or a slow vector query
| it finds nothing and says why, and the caller answers from keywords.
| A small workspace is scanned exactly; a large one goes through the
| HNSW index first when the model's size has one.
|
*/

package knowledge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Search sizes: chunks kept from the vector arm, the most chunks scanned
exactly, and the nearest vectors the index returns.
*/
const (
	vectorCandidates = 50
	exactMaxChunks   = 20_000
	annCandidates    = 200
	annDims          = 1024
)

/*
VectorChunk is one chunk the vector arm found, nearest first.
*/
type VectorChunk struct {
	Row        models.SearchKnowledgeChunksVectorRow
	Similarity float64
}

/*
VectorDocumentChunk is one chunk of a source document the vector arm
found.
*/
type VectorDocumentChunk struct {
	Row        models.SearchKnowledgeDocumentChunksVectorRow
	Similarity float64
}

/*
VectorSearch is what the vector arm found, among articles and among
source documents, the model and key the query was embedded with, what
embedding it cost and, when it found nothing on purpose, why.
*/
type VectorSearch struct {
	Chunks    []VectorChunk
	Documents []VectorDocumentChunk
	Model     string
	KeyID     *ids.UUID
	Usage     *contracts.LLMUsage
	Degraded  string
	Timings   map[string]time.Duration
}

/*
SearchChunks ranks the workspace's live chunks of the visibilities by
meaning. Only a canceled context is an error.
*/
func (v *Vectors) SearchChunks(ctx context.Context, ws ids.UUID, query string,
	opts contracts.KnowledgeSearchOptions,
) (VectorSearch, error) {
	out := VectorSearch{Timings: map[string]time.Duration{}}
	started := v.Clock.Now()
	qv, err := v.queryVector(ctx, ws, query, v.queryTimeout(opts.QueryEmbedTimeout))
	out.Timings["embed_query"] = v.Clock.Now().Sub(started)
	if err != nil {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		out.Degraded = degradedFor(err)
		return out, nil
	}
	out.Model, out.KeyID, out.Usage = qv.model, qv.keyID, qv.usage
	started = v.Clock.Now()
	rows, docs, err := v.nearest(ctx, ws, qv, opts)
	out.Timings["vector"] = v.Clock.Now().Sub(started)
	if err != nil {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		out.Degraded = "vector_timeout"
		return out, nil
	}
	for _, r := range rows {
		out.Chunks = append(out.Chunks, VectorChunk{Row: r, Similarity: 1 - r.Distance})
	}
	for _, r := range docs {
		out.Documents = append(out.Documents, VectorDocumentChunk{Row: r, Similarity: 1 - r.Distance})
	}
	return out, nil
}

/*
nearest runs the vector queries, over articles and over source
documents, under their own statement timeout.
*/
func (v *Vectors) nearest(ctx context.Context, ws ids.UUID, qv queryVector, opts contracts.KnowledgeSearchOptions,
) ([]models.SearchKnowledgeChunksVectorRow, []models.SearchKnowledgeDocumentChunksVectorRow, error) {
	var rows []models.SearchKnowledgeChunksVectorRow
	var docs []models.SearchKnowledgeDocumentChunksVectorRow
	embedding := database.NewHalfVector(qv.vector)
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		ms := v.vectorTimeout().Milliseconds()
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", ms)); err != nil {
			return err
		}
		hashes, err := v.annHashes(ctx, tx, ws, qv)
		if err != nil {
			return err
		}
		rows, err = q.SearchKnowledgeChunksVector(ctx, models.SearchKnowledgeChunksVectorParams{
			Embedding: embedding, ModelKey: qv.model, WorkspaceID: ws, Visibilities: visibilityNames(opts.Visibility),
			Hashes: hashes, RowLimit: vectorCandidates,
		})
		if err != nil {
			return err
		}
		docs, err = nearestDocuments(ctx, q, nearby{ws: ws, qv: qv, opts: opts, hashes: hashes})
		return err
	})
	return rows, docs, err
}

/*
annHashes narrows a large workspace to the hashes the HNSW index finds
nearest; a small one, or a size with no index, is scanned exactly.
*/
func (v *Vectors) annHashes(ctx context.Context, tx *database.Tx, ws ids.UUID, qv queryVector) ([]string, error) {
	q := models.New(tx)
	if model, ok := modelKeyOf(qv.model); !ok || model.Dims != annDims {
		return []string{}, nil
	}
	n, err := q.CountKnowledgeChunks(ctx, ws)
	if err != nil || n <= exactMaxChunks {
		return []string{}, err
	}
	for _, set := range []string{"SET LOCAL hnsw.ef_search = 100", "SET LOCAL hnsw.iterative_scan = relaxed_order"} {
		if _, err := tx.Exec(ctx, set); err != nil {
			return nil, err
		}
	}
	return q.SearchKnowledgeEmbeddingsANN1024(ctx, models.SearchKnowledgeEmbeddingsANN1024Params{
		Embedding: database.NewHalfVector(qv.vector), WorkspaceID: ws, ModelKey: qv.model, RowLimit: annCandidates,
	})
}

/*
degradedFor names why a search went without vectors.
*/
func degradedFor(err error) string {
	switch {
	case errors.Is(err, ErrNoEmbeddings), errors.Is(err, ErrKeyRemoved):
		return "no_embeddings_key"
	case errors.Is(err, ErrEmbedTimeout):
		return "embedding_timeout"
	}
	return "embedding_error"
}

/*
visibilityNames are the visibilities asked for, public alone when
none are.
*/
func visibilityNames(vis []contracts.KnowledgeVisibility) []string {
	if len(vis) == 0 {
		return []string{string(contracts.KnowledgeVisibilityPublic)}
	}
	out := make([]string, len(vis))
	for i, v := range vis {
		out[i] = string(v)
	}
	return out
}
