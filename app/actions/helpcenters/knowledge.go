/*
|--------------------------------------------------------------------------
| Help Center Knowledge
|--------------------------------------------------------------------------
|
| The agent answers from the workspace's published articles, public and
| agent-only alike, in every live help center. A search finds the
| articles sharing words with the customer's question, then ranks their
| sections and returns the best passages, each with the article's public
| address so the agent can cite it. With an embedding model bound, the
| same search also ranks the stored chunks by meaning and fuses both
| rankings, so a question asked in other words still finds its answer;
| when vectors are unavailable it answers from keywords, as before.
| Drafts never count: only what was published. A workspace with no
| published article yet falls back to the seeded index, so a new
| install still has something to answer from.
|
*/

package helpcenters

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
candidates is how many articles a search ranks sections of.
*/
const candidates = 12

/*
defaultPerDocument is how many chunks of one article a hybrid result
keeps.
*/
const defaultPerDocument = 3

/*
Knowledge is the agent's index over published articles. Vectors, when
set, adds the meaning half of a hybrid search.
*/
type Knowledge struct {
	DB        *database.DB
	Addresses Addresses
	Fallback  contracts.Knowledge
	Vectors   *knowledge.Vectors
}

/*
Search returns the best passages of the workspace's published articles
and of the sources feeding the search's surface.
*/
func (k *Knowledge) Search(ctx context.Context, ws ids.UUID, query string, opts contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	terms := searchTerms(append([]string{query}, opts.Queries...))
	if terms == "" {
		return contracts.KnowledgeSearchResult{Mode: "keyword"}, nil
	}
	found, err := k.candidates(ctx, ws, terms, opts)
	if err != nil {
		return contracts.KnowledgeSearchResult{}, fmt.Errorf("search articles: %w", err)
	}
	if found.count == 0 && k.Fallback != nil {
		return k.Fallback.Search(ctx, ws, query, opts)
	}
	docs := make([]knowledge.Article, 0, len(found.articles)+len(found.documents))
	for _, r := range found.articles {
		doc := k.articleOf(models.FindWorkspaceArticleDocumentRow{
			ID: r.ID, HelpCenterID: r.HelpCenterID, Subdomain: r.Subdomain, Title: r.Title, Slug: r.Slug,
			BodyMarkdown: r.BodyMarkdown, Visibility: r.Visibility, CreatedAt: r.CreatedAt,
		})
		doc.VersionID = r.VersionID
		docs = append(docs, doc)
	}
	for _, r := range found.documents {
		docs = append(docs, sourceDocumentOf(r))
	}
	res, err := knowledge.NewSeed(docs).Search(ctx, ws, query, opts)
	if err != nil || k.Vectors == nil || opts.Mode == "keyword" {
		return res, err
	}
	return k.hybrid(ctx, search{ws: ws, query: query, opts: opts, restrictions: found.restrictions}, res)
}

/*
found is what the keyword arm loads: the matching articles the
audience rule lets through, the matching source documents, how much
content the workspace has at all, and the audience restrictions, nil
when the search is not filtered by audience.
*/
type found struct {
	articles     []models.SearchWorkspaceArticlesRow
	documents    []models.SearchKnowledgeSourceDocumentsRow
	count        int64
	restrictions *audiences.Restrictions
}

/*
candidates loads the articles and source documents sharing words with
the query.
*/
func (k *Knowledge) candidates(ctx context.Context, ws ids.UUID, terms string, opts contracts.KnowledgeSearchOptions,
) (found, error) {
	var out found
	err := k.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		articles, err := q.CountWorkspaceArticles(ctx, ws)
		if err != nil {
			return err
		}
		sources, err := q.CountKnowledgeSourceDocuments(ctx, models.CountKnowledgeSourceDocumentsParams{
			WorkspaceID: ws, Target: knowledge.Target(opts.Surface),
		})
		if err != nil {
			return err
		}
		if out.count = articles + sources; out.count == 0 {
			return nil
		}
		if out.documents, err = sourceCandidates(ctx, q, asked{ws: ws, terms: terms, opts: opts}); err != nil {
			return err
		}
		if opts.ExcludeArticles || articles == 0 {
			return nil
		}
		return articleCandidates(ctx, q, asked{ws: ws, terms: terms, opts: opts}, &out)
	})
	return out, err
}

/*
asked is what a keyword search looks for: the workspace, the query's
words and the search's options.
*/
type asked struct {
	ws    ids.UUID
	terms string
	opts  contracts.KnowledgeSearchOptions
}

/*
articleCandidates loads the matching articles, dropping those the
audience rule hides.
*/
func articleCandidates(ctx context.Context, q *models.Queries, a asked, out *found) error {
	ws, terms, opts := a.ws, a.terms, a.opts

	rows, err := q.SearchWorkspaceArticles(ctx, models.SearchWorkspaceArticlesParams{
		Query: terms, WorkspaceID: ws, Visibilities: visibilities(opts.Visibility), RowLimit: candidates,
	})
	if err != nil || !opts.AudienceFiltered {
		out.articles = rows
		return err
	}
	r, err := audiences.LoadRestrictions(ctx, q, ws)
	if err != nil {
		return err
	}
	out.restrictions = &r
	out.articles = slices.DeleteFunc(rows, func(row models.SearchWorkspaceArticlesRow) bool {
		return !r.Article(row.ID, opts.AllowedAudienceIDs)
	})
	return nil
}

/*
sourceCandidates loads the matching documents of the sources feeding
the search's surface, unless the search leaves sources out.
*/
func sourceCandidates(ctx context.Context, q *models.Queries, a asked,
) ([]models.SearchKnowledgeSourceDocumentsRow, error) {
	ws, terms, opts := a.ws, a.terms, a.opts

	if opts.ExcludeSources {
		return nil, nil
	}
	return q.SearchKnowledgeSourceDocuments(ctx, models.SearchKnowledgeSourceDocumentsParams{
		Query: terms, WorkspaceID: ws, Target: knowledge.Target(opts.Surface),
		Visibilities: visibilities(opts.Visibility), SourceIds: orNone(opts.SourceIDs), RowLimit: candidates,
	})
}

/*
sourceDocumentOf is one source document as the index chunks it. Only a
public document carries its address, so the agent never links one
meant for its eyes only.
*/
func sourceDocumentOf(r models.SearchKnowledgeSourceDocumentsRow) knowledge.Article {
	doc := knowledge.Article{
		Title: r.Title, Body: r.ContentMarkdown, DocumentID: r.ID, SourceID: &r.SourceID, SourceType: r.SourceType,
		VersionID: knowledge.DocumentVersion(r.ID, r.ContentHash), Locale: r.Locale,
		Visibility: contracts.KnowledgeVisibility(r.Visibility),
	}
	if r.Url != nil && r.Visibility == string(contracts.KnowledgeVisibilityPublic) {
		doc.URL = *r.Url
	}
	if r.IndexedAt != nil {
		doc.UpdatedAt = *r.IndexedAt
	}
	return doc
}

/*
orNone is the list, or an empty one for none.
*/
func orNone(list []ids.UUID) []ids.UUID {
	if list == nil {
		return []ids.UUID{}
	}
	return list
}

/*
Document returns one published article's Markdown.
*/
func (k *Knowledge) Document(ctx context.Context, ws, documentID ids.UUID, vis []contracts.KnowledgeVisibility,
) (contracts.KnowledgeDocument, error) {
	var out contracts.KnowledgeDocument
	err := k.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		r, err := q.FindWorkspaceArticleDocument(ctx, models.FindWorkspaceArticleDocumentParams{
			WorkspaceID: ws, ID: documentID, Visibilities: visibilities(vis),
		})
		if err != nil {
			return err
		}
		doc := k.articleOf(r)
		out = contracts.KnowledgeDocument{
			ID: r.ID, SourceType: "article", Title: r.Title, URL: doc.URL, Text: r.BodyMarkdown,
			Visibility: contracts.KnowledgeVisibility(r.Visibility), Locale: "en", UpdatedAt: r.CreatedAt,
		}
		return nil
	})
	if errors.Is(err, database.ErrNotFound) {
		out, err = k.sourceDocument(ctx, ws, documentID, vis)
	}
	if err != nil && k.Fallback != nil {
		return k.Fallback.Document(ctx, ws, documentID, vis)
	}
	return out, err
}

/*
sourceDocument returns one live document of a synced source, its
address only when it is public.
*/
func (k *Knowledge) sourceDocument(ctx context.Context, ws, documentID ids.UUID, vis []contracts.KnowledgeVisibility,
) (contracts.KnowledgeDocument, error) {
	var out contracts.KnowledgeDocument
	err := k.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		r, err := q.FindKnowledgeSourceDocumentForReading(ctx, models.FindKnowledgeSourceDocumentForReadingParams{
			WorkspaceID: ws, ID: documentID, Visibilities: visibilities(vis),
		})
		if err != nil {
			return err
		}
		out = contracts.KnowledgeDocument{
			ID: r.ID, SourceType: r.SourceType, Title: r.Title, Text: r.ContentMarkdown,
			Visibility: contracts.KnowledgeVisibility(r.Visibility), Locale: cmp.Or(r.Locale, "en"),
		}
		if r.Url != nil && out.Visibility == contracts.KnowledgeVisibilityPublic {
			out.URL = *r.Url
		}
		if r.IndexedAt != nil {
			out.UpdatedAt = *r.IndexedAt
		}
		return nil
	})
	return out, err
}

/*
Status reports whether the workspace has published articles, else what
the fallback has.
*/
func (k *Knowledge) Status(ctx context.Context, ws ids.UUID) (contracts.KnowledgeStatus, error) {
	var count int64
	err := k.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		articles, err := q.CountWorkspaceArticles(ctx, ws)
		if err != nil {
			return err
		}
		sources, err := q.CountKnowledgeSourceDocuments(ctx, models.CountKnowledgeSourceDocumentsParams{
			WorkspaceID: ws, Target: knowledge.Target("agent"),
		})
		count = articles + sources
		return err
	})
	if err != nil {
		return contracts.KnowledgeStatus{}, fmt.Errorf("count articles: %w", err)
	}
	if count == 0 && k.Fallback != nil {
		return k.Fallback.Status(ctx, ws)
	}
	status := contracts.KnowledgeStatus{HasContent: count > 0, DocumentCount: count}
	if k.Vectors != nil {
		if status.EmbeddingModel, err = k.Vectors.ActiveModel(ctx, ws); err != nil {
			return status, err
		}
	}
	return status, nil
}

/*
EmbeddingKeyInUse reports whether the workspace embeds through the
provider key.
*/
func (k *Knowledge) EmbeddingKeyInUse(ctx context.Context, ws, keyID ids.UUID) (bool, error) {
	if k.Vectors == nil {
		return false, nil
	}
	return k.Vectors.KeyInUse(ctx, ws, keyID)
}

/*
PublicHosts lists the hosts of the workspace's help centers, whose
links the agent may emit.
*/
func (k *Knowledge) PublicHosts(ctx context.Context, ws ids.UUID) ([]string, error) {
	var subs []string
	err := k.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		subs, err = q.ListWorkspaceHelpCenterSubdomains(ctx, ws)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list help center hosts: %w", err)
	}
	seen := map[string]bool{}
	var out []string
	for _, sub := range subs {
		u, err := url.Parse(k.Addresses.Root(models.HelpCenter{Subdomain: sub}))
		if err == nil && !seen[u.Host] {
			seen[u.Host] = true
			out = append(out, u.Host)
		}
	}
	return out, nil
}

/*
articleOf is one published article as the index chunks it.
*/
func (k *Knowledge) articleOf(r models.FindWorkspaceArticleDocumentRow) knowledge.Article {
	return knowledge.Article{
		Title: r.Title, Body: r.BodyMarkdown, DocumentID: r.ID, ArticleID: &r.ID, HelpCenterID: &r.HelpCenterID,
		URL:        k.Addresses.Root(models.HelpCenter{Subdomain: r.Subdomain}) + "/articles/" + r.Slug,
		Visibility: contracts.KnowledgeVisibility(r.Visibility), UpdatedAt: r.CreatedAt,
	}
}

func (k *Knowledge) inWorkspace(ctx context.Context, ws ids.UUID, fn func(context.Context, *models.Queries) error,
) error {
	return k.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}

/*
visibilities are the visibilities asked for, public alone when none are.
*/
func visibilities(vis []contracts.KnowledgeVisibility) []string {
	if len(vis) == 0 {
		return []string{string(contracts.KnowledgeVisibilityPublic)}
	}
	out := make([]string, len(vis))
	for i, v := range vis {
		out[i] = string(v)
	}
	return out
}

/*
searchTerms joins the words of the queries into a query matching any of
them; words are only letters and digits, so nothing a customer types
can change the query's shape.
*/
func searchTerms(queries []string) string {
	words := regexp.MustCompile(`[\p{L}\p{N}]+`)
	seen := map[string]bool{}
	var out []string
	for _, q := range queries {
		for _, w := range words.FindAllString(strings.ToLower(q), -1) {
			if len(w) > 1 && !seen[w] && len(out) < 32 {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return strings.Join(out, " | ")
}
