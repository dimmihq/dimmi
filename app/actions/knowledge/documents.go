/*
|--------------------------------------------------------------------------
| Source Document Index
|--------------------------------------------------------------------------
|
| A knowledge source's documents are chunked exactly like articles and
| their vectors live in the same store, keyed by the hash of what was
| embedded, so a document that did not change costs nothing and two
| documents saying the same thing share one vector. A document's chunks
| follow its current text inside the transaction that stored it; the
| vectors follow in the background for every model the workspace embeds
| with. Each source says who it feeds: the agent, the Operator, or both,
| and a search only sees the sources feeding its surface.
|
*/

package knowledge

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxDocumentChunks is the most chunks one document is cut into; a
longer document keeps its first ones.
*/
const maxDocumentChunks = 500

/*
SourceDocument is a source's document as the index cuts it.
*/
type SourceDocument struct {
	ID          ids.UUID
	WorkspaceID ids.UUID
	SourceID    ids.UUID
	Title       string
	Markdown    string
	ContentHash string
	Visibility  contracts.KnowledgeVisibility
}

/*
DocumentVersion is what the chunks of a document's current text derive
their ids from, so the keyword and vector arms name a chunk alike.
*/
func DocumentVersion(documentID ids.UUID, contentHash string) ids.UUID {
	return ids.Derive(documentID, "text:"+contentHash)
}

/*
Target is the source target a search surface reads: the Operator reads
the sources feeding it, every other surface those feeding the agent.
*/
func Target(surface string) string {
	if surface == "operator" {
		return "operator"
	}

	return "agent"
}

/*
IndexDocument replaces the document's chunks with those of its current
text and queues the vectors they lack, inside the transaction. It
returns how many chunks the document has.
*/
func (v *Vectors) IndexDocument(ctx context.Context, tx *database.Tx, d SourceDocument) (int, error) {
	q := models.New(tx)
	if err := q.DeleteKnowledgeDocumentChunks(ctx, models.DeleteKnowledgeDocumentChunksParams{
		WorkspaceID: d.WorkspaceID, DocumentID: d.ID,
	}); err != nil {
		return 0, fmt.Errorf("drop document chunks: %w", err)
	}

	version := DocumentVersion(d.ID, d.ContentHash)
	chunks := ChunkArticle(Article{Title: d.Title, Body: d.Markdown, VersionID: version, Visibility: d.Visibility})
	chunks = chunks[:min(len(chunks), maxDocumentChunks)]
	now := v.Clock.Now()

	for i, c := range chunks {
		err := q.InsertKnowledgeDocumentChunk(ctx, models.InsertKnowledgeDocumentChunkParams{
			ID: c.ID, WorkspaceID: d.WorkspaceID, DocumentID: d.ID, SourceID: d.SourceID, Ordinal: int32(i),
			Title: c.Title, HeadingPath: c.HeadingPath, Body: c.Text, Visibility: string(c.Visibility),
			EmbedHash: EmbedHash(EmbedInput(c.Title, c.HeadingPath, c.Text)), Now: now,
		})
		if err != nil {
			return 0, fmt.Errorf("store document chunk: %w", err)
		}
	}

	return len(chunks), v.queueDocument(ctx, tx, d)
}

/*
queueDocument queues the document's missing vectors for every model
the workspace embeds with.
*/
func (v *Vectors) queueDocument(ctx context.Context, tx *database.Tx, d SourceDocument) error {
	s, err := v.ensure(ctx, tx, d.WorkspaceID)
	if err != nil {
		return err
	}

	q := models.New(tx)
	for _, model := range embeddingModels(s) {
		hashes, err := q.ListDocumentEmbedHashes(ctx, models.ListDocumentEmbedHashesParams{
			WorkspaceID: d.WorkspaceID, DocumentID: d.ID, ModelKey: model,
		})
		if err != nil {
			return fmt.Errorf("list document hashes: %w", err)
		}

		args := jobs.EmbedKnowledgeChunksArgs{WorkspaceID: d.WorkspaceID, ModelKey: model}
		if err := v.pushBatches(ctx, tx, args, hashes); err != nil {
			return err
		}
	}

	return nil
}

/*
nearby is one vector search: the workspace, the query's vector, the
search's options and the hashes the index narrowed it to.
*/
type nearby struct {
	ws     ids.UUID
	qv     queryVector
	opts   contracts.KnowledgeSearchOptions
	hashes []string
}

/*
nearestDocuments ranks the live chunks of the sources feeding the
search's surface by meaning, unless the search leaves sources out.
*/
func nearestDocuments(ctx context.Context, q *models.Queries, n nearby,
) ([]models.SearchKnowledgeDocumentChunksVectorRow, error) {
	ws, qv, opts, hashes := n.ws, n.qv, n.opts, n.hashes
	if opts.ExcludeSources {
		return nil, nil
	}

	return q.SearchKnowledgeDocumentChunksVector(ctx, models.SearchKnowledgeDocumentChunksVectorParams{
		Embedding: database.NewHalfVector(qv.vector), ModelKey: qv.model, WorkspaceID: ws,
		Target: Target(opts.Surface), Visibilities: visibilityNames(opts.Visibility), Hashes: hashes,
		RowLimit: vectorCandidates,
	})
}
