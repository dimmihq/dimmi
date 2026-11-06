/*
|--------------------------------------------------------------------------
| Knowledge Stub
|--------------------------------------------------------------------------
|
| Until the help center's index exists, the agent retrieves from this
| stand-in: a handful of canned chunks per workspace, so the whole
| pipeline — answerability, citations, unknown — already works end to
| end. When the real index lands it replaces this file behind the
| same contract and nothing above it changes.
|
*/

/*
Package knowledge is the dev stub of the knowledge index.
*/
package knowledge

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Stub answers every search with its canned chunks. Content reports
ready so the agent can be switched on before the index exists.
*/
type Stub struct {
	chunks []contracts.KnowledgeChunk
}

/*
NewStub returns a stub serving the canned chunks.
*/
func NewStub() *Stub {
	return &Stub{chunks: cannedChunks()}
}

/*
Search returns the canned chunks ranked in order, capped at the
requested limit.
*/
func (s *Stub) Search(_ context.Context, _ ids.UUID, _ string,
	opts contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	limit := opts.Limit
	if limit <= 0 || limit > len(s.chunks) {
		limit = min(max(limit, 0), len(s.chunks))
	}
	if limit == 0 && len(s.chunks) > 0 {
		limit = len(s.chunks)
	}

	chunks := make([]contracts.KnowledgeChunk, limit)
	copy(chunks, s.chunks[:limit])

	return contracts.KnowledgeSearchResult{Chunks: chunks, Mode: "keyword"}, nil
}

/*
Document returns the canned document text.
*/
func (s *Stub) Document(_ context.Context, _, documentID ids.UUID,
	_ []contracts.KnowledgeVisibility,
) (contracts.KnowledgeDocument, error) {
	for _, chunk := range s.chunks {
		if chunk.DocumentID == documentID {
			return contracts.KnowledgeDocument{
				ID:         chunk.DocumentID,
				SourceType: chunk.SourceType,
				Title:      chunk.Title,
				URL:        chunk.URL,
				Text:       chunk.Text,
				Visibility: chunk.Visibility,
			}, nil
		}
	}
	return contracts.KnowledgeDocument{}, nil
}

/*
Status reports content ready so the agent's readiness check passes.
*/
func (s *Stub) Status(_ context.Context, _ ids.UUID) (contracts.KnowledgeStatus, error) {
	return contracts.KnowledgeStatus{
		HasContent: true, DocumentCount: int64(len(s.chunks)), ChunkCount: int64(len(s.chunks)),
	}, nil
}

/*
EmbeddingKeyInUse never holds a key.
*/
func (s *Stub) EmbeddingKeyInUse(_ context.Context, _, _ ids.UUID) (bool, error) {
	return false, nil
}

/*
PublicHosts lists the hosts the canned chunks cite.
*/
func (s *Stub) PublicHosts(_ context.Context, _ ids.UUID) ([]string, error) {
	return []string{}, nil
}

/*
cannedChunks is the stand-in corpus: one plausible support article
per retrieval path the pipeline exercises.
*/
func cannedChunks() []contracts.KnowledgeChunk {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sim := 0.8

	return []contracts.KnowledgeChunk{
		{
			ID:         ids.New(),
			DocumentID: ids.New(),
			SourceType: "article",
			Title:      "Change your billing details",
			URL:        "https://help.example.com/billing",
			Text: "Open Settings > Billing. There you can update the billing email, " +
				"card and invoice address. Changes apply to the next invoice.",
			Similarity: &sim,
			Score:      1,
			Rank:       1,
			MatchedBy:  []string{"fts"},
			Visibility: contracts.KnowledgeVisibilityPublic,
			Locale:     "en",
			UpdatedAt:  at,
		},
		{
			ID:         ids.New(),
			DocumentID: ids.New(),
			SourceType: "article",
			Title:      "Cancel your subscription",
			URL:        "https://help.example.com/cancel",
			Text: "You can cancel any time from Settings > Billing > Cancel plan. " +
				"The plan stays active until the end of the paid period.",
			Similarity: &sim,
			Score:      0.9,
			Rank:       2,
			MatchedBy:  []string{"fts"},
			Visibility: contracts.KnowledgeVisibilityPublic,
			Locale:     "en",
			UpdatedAt:  at,
		},
	}
}
