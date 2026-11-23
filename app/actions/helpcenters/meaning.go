/*
|--------------------------------------------------------------------------
| Search By Meaning
|--------------------------------------------------------------------------
|
| How a workspace with an embedding model is searched. Two searches
| find candidates: the question's vector finds the passages nearest in
| meaning, and its words find the passages that say the same thing in
| the same terms, which is how an error code, a plan name or a tracking
| number is found exactly. Neither decides the order. A reranker reads
| the question beside every candidate and puts the ones that answer it
| first, so a passage that merely shares a word never outranks one that
| answers. Vector hits pass the same audience rule as keyword hits, and
| carry the article's public address the same way.
|
| With no reranker, or one that failed or ran out of time, the two
| rankings are fused by rank: a passage both searches found rises.
|
| Keywords alone are the last resort: only when the question's vector
| could not be had, after its retry, or nothing in the workspace is
| embedded yet, does the caller answer from keywords, and the result
| says why.
|
*/

package helpcenters

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Candidate counts: how many of the nearest passages and how many of the
best keyword passages the reranker reads. Together they are enough to
hold every passage worth returning and few enough to read in a
fraction of a second. A search that asked for no number of passages
gets defaultLimit.
*/
const (
	rerankCandidates  = 40
	keywordCandidates = 20
	defaultLimit      = 8
)

/*
defaultRerankTimeout bounds the reranker when the install set no wait.
*/
const defaultRerankTimeout = 700 * time.Millisecond

/*
nothingEmbedded is why a search by meaning found nothing to rank: the
workspace has no vectors of its model yet.
*/
const nothingEmbedded = "nothing_embedded"

/*
search is one search by meaning: the workspace, the query, its options
and the audience restrictions, nil when the search is not filtered by
audience.
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
ranked ranks what the search by meaning and the keyword search found.
When the search by meaning could not be done, the result holds no
passages and its Degraded says why, and the caller answers from
keywords.
*/
func (k *Knowledge) ranked(ctx context.Context, s search, vec knowledge.VectorSearch,
	keyword []contracts.KnowledgeChunk,
) (contracts.KnowledgeSearchResult, error) {
	out := contracts.KnowledgeSearchResult{
		Mode: "hybrid", Degraded: vec.Degraded, Timings: vec.Timings,
		EmbeddingModel: vec.Model, EmbeddingKeyID: vec.KeyID, EmbeddingUsage: vec.Usage,
	}
	if vec.Degraded != "" {
		return out, nil
	}
	var err error
	if s.restrictions, err = k.restrictions(ctx, s); err != nil {
		return contracts.KnowledgeSearchResult{}, err
	}
	nearest := documentChunks(vec.Documents)
	if !s.opts.ExcludeArticles {
		nearest = append(nearest, k.vectorChunks(s, vec.Chunks)...)
	}
	if len(nearest) == 0 {
		out.Degraded = nothingEmbedded
		return out, nil
	}
	slices.SortStableFunc(nearest, func(a, b contracts.KnowledgeChunk) int {
		return cmp.Compare(*b.Similarity, *a.Similarity)
	})
	nearest = nearest[:min(len(nearest), rerankCandidates)]
	limit := cmp.Or(max(s.opts.Limit, 0), defaultLimit)
	perDoc := cmp.Or(max(s.opts.MaxPerDocument, 0), defaultPerDocument)

	started := k.Vectors.Clock.Now()
	order, reranked := k.rerank(ctx, s.query, pooled(nearest, keyword))
	if k.Reranker != nil {
		out.Timings["rerank"] = k.Vectors.Clock.Now().Sub(started)
	}
	if reranked {
		out.Mode, out.Chunks = "reranked", knowledge.Fuse(limit, perDoc, order)
		return out, nil
	}
	out.Chunks = knowledge.Fuse(limit, perDoc, nearest, keyword)
	return out, nil
}

/*
pooled are the passages the reranker reads: the nearest ones, then
the keyword passages the search by meaning did not find. A passage both
found is read once and keeps what each search knew of it.
*/
func pooled(nearest, keyword []contracts.KnowledgeChunk) []contracts.KnowledgeChunk {
	out := slices.Clone(nearest)
	at := make(map[ids.UUID]int, len(out))
	for i, ch := range out {
		at[ch.ID] = i
	}
	for _, ch := range keyword {
		i, both := at[ch.ID]
		if !both {
			out = append(out, ch)
			continue
		}
		out[i].Coverage = ch.Coverage
		for _, by := range ch.MatchedBy {
			if !slices.Contains(out[i].MatchedBy, by) {
				out[i].MatchedBy = append(slices.Clone(out[i].MatchedBy), by)
			}
		}
	}
	return out
}

/*
restrictions loads the audience rule of a search filtered by audience.
*/
func (k *Knowledge) restrictions(ctx context.Context, s search) (*audiences.Restrictions, error) {
	if !s.opts.AudienceFiltered || s.opts.ExcludeArticles {
		return nil, nil
	}
	var out audiences.Restrictions
	err := k.inWorkspace(ctx, s.ws, func(ctx context.Context, q *models.Queries) (err error) {
		out, err = audiences.LoadRestrictions(ctx, q, s.ws)
		return err
	})
	return &out, err
}

/*
rerank puts the passages that answer the query first. With no
reranker, or one that failed or ran out of time, it reports false and
the caller orders them another way.
*/
func (k *Knowledge) rerank(ctx context.Context, query string, chunks []contracts.KnowledgeChunk,
) ([]contracts.KnowledgeChunk, bool) {
	if k.Reranker == nil || len(chunks) < 2 {
		return chunks, false
	}
	head := chunks[:min(len(chunks), contracts.MaxRerankDocuments)]
	documents := make([]string, len(head))
	for i, ch := range head {
		documents[i] = passage(ch)
	}
	ctx, cancel := context.WithTimeout(ctx, cmp.Or(k.RerankTimeout, defaultRerankTimeout))
	defer cancel()
	res, err := k.Reranker.Rerank(ctx, contracts.RerankRequest{Query: query, Documents: documents})
	if err != nil {
		return chunks, false
	}
	out := make([]contracts.KnowledgeChunk, 0, len(chunks))
	placed := make([]bool, len(head))
	for _, r := range res.Results {
		if r.Index < 0 || r.Index >= len(head) || placed[r.Index] {
			continue
		}
		placed[r.Index] = true
		out = append(out, head[r.Index])
	}
	for i, ch := range head {
		if !placed[i] {
			out = append(out, ch)
		}
	}
	return append(out, chunks[len(head):]...), true
}

/*
passage is what the reranker reads of a chunk: its title and headings,
which often name the subject the body leaves out, then the body.
*/
func passage(ch contracts.KnowledgeChunk) string {
	head := strings.TrimSpace(ch.Title + "\n" + strings.Join(ch.HeadingPath, " > "))
	return strings.TrimSpace(head + "\n\n" + ch.Text)
}

/*
vectorChunks turns the vector search's rows into chunks, dropping
those the audience rule hides.
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
documentChunks turns the vector search's source document rows into
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
