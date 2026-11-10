/*
|--------------------------------------------------------------------------
| Seeded Knowledge Index
|--------------------------------------------------------------------------
|
| Until the help center's index exists, the agent retrieves from a
| small real index: Markdown articles read from one directory at boot,
| cut into one chunk per heading and searched in memory with BM25. The
| same articles serve every workspace, which suits development and
| demos. With no directory configured the index is empty and the agent
| honestly knows nothing.
|
| Similarity is the share of the query's weight a chunk covers, from 0
| to 1, so an unrelated question scores near zero and the pipeline's
| minimum similarity keeps the agent from answering it.
|
*/

package knowledge

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Article is one document to index: a title, its public URL and Markdown.
Documents from a help center also carry their article, the published
version the chunks are cut from, help center and who may see them;
seed documents leave those empty and are public.
*/
type Article struct {
	Title        string
	URL          string
	Body         string
	DocumentID   ids.UUID
	VersionID    ids.UUID
	ArticleID    *ids.UUID
	HelpCenterID *ids.UUID
	Visibility   contracts.KnowledgeVisibility
	UpdatedAt    time.Time
}

/*
Seed is the in-memory index over the seed articles. It is immutable
after NewSeed and safe for concurrent use.
*/
type Seed struct {
	chunks []indexed
	stats  corpus
}

/*
LoadSeed reads every .md file under the directory, opened as a local
disk of the storage manager, into an index. An empty path is an empty
index.
*/
func LoadSeed(ctx context.Context, storage *filesystem.Manager, path string) (*Seed, error) {
	if path == "" {
		return NewSeed(nil), nil
	}

	disk, err := storage.Build("knowledge_seed", config.Section{"driver": "local", "root": path})
	if err != nil {
		return nil, fmt.Errorf("open knowledge seed %s: %w", path, err)
	}
	if closer, ok := disk.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	articles, err := readArticles(ctx, disk)
	if err != nil {
		return nil, fmt.Errorf("read knowledge seed %s: %w", path, err)
	}
	return NewSeed(articles), nil
}

/*
readArticles parses every Markdown file on the disk, in path order.
*/
func readArticles(ctx context.Context, disk filesystem.Filesystem) ([]Article, error) {
	var paths []string
	for entry, err := range disk.List(ctx, "") {
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(entry.Path, ".md") {
			paths = append(paths, entry.Path)
		}
	}
	slices.Sort(paths)

	articles := make([]Article, 0, len(paths))
	for _, path := range paths {
		r, err := disk.Open(ctx, path)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return nil, err
		}
		articles = append(articles, ParseArticle(path, string(data)))
	}
	return articles, nil
}

/*
NewSeed indexes the articles.
*/
func NewSeed(articles []Article) *Seed {
	s := &Seed{}
	for _, a := range articles {
		doc := a.DocumentID
		if doc == (ids.UUID{}) {
			doc = ids.New()
		}
		for _, c := range ChunkArticle(a) {
			c.DocumentID = doc
			s.chunks = append(s.chunks, index(c))
		}
	}
	s.stats = stats(s.chunks)
	return s
}

/*
Search ranks the chunks by BM25 over the query and the extra queries,
keeping each chunk's best similarity across them, and drops chunks
that match nothing.
*/
func (s *Seed) Search(_ context.Context, _ ids.UUID, query string,
	opts contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	queries := append([]string{query}, opts.Queries...)
	hits := s.rank(queries, opts.Visibility)

	limit := opts.Limit
	if limit <= 0 {
		limit = 8
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}

	out := make([]contracts.KnowledgeChunk, len(hits))
	for i, h := range hits {
		chunk := s.chunks[h.at].chunk
		similarity := h.similarity
		chunk.Similarity, chunk.Coverage = &similarity, &similarity
		chunk.Score = h.score
		chunk.Rank = i + 1
		chunk.MatchedBy = []string{"bm25"}
		out[i] = chunk
	}
	return contracts.KnowledgeSearchResult{Chunks: out, Mode: "keyword"}, nil
}

/*
Document returns one seed document's chunks joined back together.
*/
func (s *Seed) Document(_ context.Context, _, documentID ids.UUID,
	_ []contracts.KnowledgeVisibility,
) (contracts.KnowledgeDocument, error) {
	doc := contracts.KnowledgeDocument{ID: documentID}
	var parts []string
	for _, c := range s.chunks {
		if c.chunk.DocumentID != documentID {
			continue
		}
		doc.Title, doc.URL, doc.SourceType = c.chunk.Title, c.chunk.URL, c.chunk.SourceType
		doc.Visibility, doc.Locale = c.chunk.Visibility, c.chunk.Locale
		parts = append(parts, c.chunk.Text)
	}
	doc.Text = strings.Join(parts, "\n\n")
	return doc, nil
}

/*
Status reports whether any article was loaded.
*/
func (s *Seed) Status(_ context.Context, _ ids.UUID) (contracts.KnowledgeStatus, error) {
	docs := map[ids.UUID]bool{}
	for _, c := range s.chunks {
		docs[c.chunk.DocumentID] = true
	}
	return contracts.KnowledgeStatus{
		HasContent: len(s.chunks) > 0, DocumentCount: int64(len(docs)), ChunkCount: int64(len(s.chunks)),
	}, nil
}

/*
EmbeddingKeyInUse never holds a key: the seed index embeds nothing.
*/
func (s *Seed) EmbeddingKeyInUse(_ context.Context, _, _ ids.UUID) (bool, error) {
	return false, nil
}

/*
PublicHosts lists the hosts the seed articles live on.
*/
func (s *Seed) PublicHosts(_ context.Context, _ ids.UUID) ([]string, error) {
	var hosts []string
	for _, c := range s.chunks {
		host := hostOf(c.chunk.URL)
		if host != "" && !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	return hosts, nil
}
