/*
|--------------------------------------------------------------------------
| Hybrid Knowledge Search
|--------------------------------------------------------------------------
|
| The keyword ranking is fused with the vector ranking of the same
| chunks, so a passage found both ways rises and a passage phrased
| unlike the question still comes back. Vector hits pass the same
| audience rule as keyword hits, and carry the article's public
| address the same way. When the vector arm finds nothing on purpose,
| the keyword result is the answer, labeled with why.
|
*/

package helpcenters

import (
	"cmp"
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
search is one hybrid search: the workspace, the query, its options and
the audience restrictions the keyword arm loaded, nil when the search
is not filtered by audience.
*/
type search struct {
	ws           ids.UUID
	query        string
	opts         contracts.KnowledgeSearchOptions
	restrictions *audiences.Restrictions
}

/*
allows reports whether the audience rule lets the search see the
article.
*/
func (s search) allows(article ids.UUID) bool {
	if !s.opts.AudienceFiltered {
		return true
	}
	return s.restrictions != nil && s.restrictions.Article(article, s.opts.AllowedAudienceIDs)
}

/*
hybrid adds the vector ranking to the keyword result and fuses them.
*/
func (k *Knowledge) hybrid(ctx context.Context, s search, keyword contracts.KnowledgeSearchResult,
) (contracts.KnowledgeSearchResult, error) {
	vec, err := k.Vectors.SearchChunks(ctx, s.ws, s.query, s.opts)
	if err != nil {
		return contracts.KnowledgeSearchResult{}, err
	}
	keyword.Timings = vec.Timings
	if vec.Degraded != "" {
		keyword.Degraded = vec.Degraded
		if vec.Degraded != "no_embeddings_key" {
			keyword.Mode = "fts_fallback"
		}
		return keyword, nil
	}
	started := k.Vectors.Clock.Now()
	chunks := append(k.vectorChunks(s, vec.Chunks), documentChunks(vec.Documents)...)
	limit, perDoc := s.opts.Limit, s.opts.MaxPerDocument
	if limit <= 0 {
		limit = 8
	}
	if perDoc <= 0 {
		perDoc = defaultPerDocument
	}
	out := contracts.KnowledgeSearchResult{
		Chunks: knowledge.Fuse(limit, perDoc, keyword.Chunks, chunks), Mode: "hybrid",
		EmbeddingModel: vec.Model, EmbeddingKeyID: vec.KeyID, EmbeddingUsage: vec.Usage, Timings: vec.Timings,
	}
	out.Timings["fuse"] = k.Vectors.Clock.Now().Sub(started)
	return out, nil
}

/*
vectorChunks turns the vector arm's rows into chunks, dropping those
the audience rule hides.
*/
func (k *Knowledge) vectorChunks(s search, rows []knowledge.VectorChunk) []contracts.KnowledgeChunk {
	out := make([]contracts.KnowledgeChunk, 0, len(rows))
	for _, v := range rows {
		r := v.Row
		if !s.allows(r.ArticleID) {
			continue
		}
		doc := k.articleOf(models.FindWorkspaceArticleDocumentRow{
			ID: r.ArticleID, HelpCenterID: r.HelpCenterID, Subdomain: r.Subdomain, Title: r.Title, Slug: r.Slug,
			Visibility: r.Visibility, CreatedAt: r.CreatedAt,
		})
		similarity := v.Similarity
		chunk := contracts.KnowledgeChunk{
			ID: r.ID, DocumentID: r.ArticleID, ArticleID: doc.ArticleID, HelpCenterID: doc.HelpCenterID,
			SourceType: "article", Title: r.Title, HeadingPath: r.HeadingPath, URL: doc.URL, Text: r.Body,
			Similarity: &similarity, MatchedBy: []string{"vector"},
			Visibility: contracts.KnowledgeVisibility(r.Visibility), Locale: "en", UpdatedAt: r.CreatedAt,
		}
		out = append(out, chunk)
	}
	return out
}

/*
documentChunks turns the vector arm's source document rows into
chunks; only a public document carries its address.
*/
func documentChunks(rows []knowledge.VectorDocumentChunk) []contracts.KnowledgeChunk {
	out := make([]contracts.KnowledgeChunk, 0, len(rows))
	for _, v := range rows {
		r := v.Row
		similarity := v.Similarity
		chunk := contracts.KnowledgeChunk{
			ID: r.ID, DocumentID: r.DocumentID, SourceID: &r.SourceID, SourceType: r.SourceType, Title: r.Title,
			HeadingPath: r.HeadingPath, Text: r.Body, Similarity: &similarity, MatchedBy: []string{"vector"},
			Visibility: contracts.KnowledgeVisibility(r.Visibility), Locale: cmp.Or(r.Locale, "en"),
		}
		if r.Url != nil && chunk.Visibility == contracts.KnowledgeVisibilityPublic {
			chunk.URL = *r.Url
		}
		if r.IndexedAt != nil {
			chunk.UpdatedAt = *r.IndexedAt
		}
		out = append(out, chunk)
	}
	return out
}
