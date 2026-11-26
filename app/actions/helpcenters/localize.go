/*
|--------------------------------------------------------------------------
| What Readers Of Each Language See
|--------------------------------------------------------------------------
|
| Readers of the default language see the source. Readers of another
| language see what is translated into it, and, unless the team chose to
| hide them, the articles that are not, in the default language with a
| notice saying so. With them hidden, a collection shows in a language
| once its name is translated and it, or a collection below it, holds an
| article readers of that language can read. Every article and collection knows its address in
| each language, so a page can link its other languages and the
| language picker can lead to the same page.
|
*/

package helpcenters

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Alternates gives the slug of each article and collection in each
language readers may read it in.
*/
type Alternates struct {
	Articles    map[ids.UUID]map[string]string
	Collections map[ids.UUID]map[string]string
}

/*
Article returns the article's slug in every language it is published
in.
*/
func (a Alternates) Article(id ids.UUID) map[string]string { return a.Articles[id] }

/*
Collection returns the collection's slug in every language it shows in.
*/
func (a Alternates) Collection(id ids.UUID) map[string]string { return a.Collections[id] }

/*
ArticleAt finds the article readers reach at the slug in any language
of the help center, and that language, the default first.
*/
func (s Snapshot) ArticleAt(slug string) (ids.UUID, string, bool) {
	def := s.HelpCenter.Locale
	for id, slugs := range s.Alternates.Articles {
		if slugs[def] == slug {
			return id, def, true
		}
	}
	for id, slugs := range s.Alternates.Articles {
		for locale, at := range slugs {
			if at == slug {
				return id, locale, true
			}
		}
	}
	return ids.UUID{}, "", false
}

/*
shelves is everything published in a help center, in every language.
*/
type shelves struct {
	center       models.HelpCenter
	collections  []models.Collection
	articles     []models.ListPublishedArticlesRow
	translated   []models.ListPublishedArticleTranslationsRow
	translations []models.ListCollectionTranslationsRow
}

/*
readShelves reads a help center's collections and published articles
with all their translations.
*/
func readShelves(ctx context.Context, q *models.Queries, h models.HelpCenter) (shelves, error) {
	out := shelves{center: h}
	var err error
	out.collections, err = q.ListCollections(ctx, models.ListCollectionsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return out, fmt.Errorf("list collections: %w", err)
	}
	out.articles, err = q.ListPublishedArticles(ctx, models.ListPublishedArticlesParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return out, fmt.Errorf("list published articles: %w", err)
	}
	out.articles = refreshed(out.articles)
	if len(LocalesOf(h)) < 2 {
		return out, nil
	}
	out.translated, err = q.ListPublishedArticleTranslations(ctx, models.ListPublishedArticleTranslationsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return out, fmt.Errorf("list published translations: %w", err)
	}
	out.translations, err = q.ListCollectionTranslations(ctx, models.ListCollectionTranslationsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return out, fmt.Errorf("list collection translations: %w", err)
	}
	return out, nil
}

/*
shelf is what readers of one language see: its collections and
articles, and which of those articles are shown in the default language
because they are not translated into this one.
*/
type shelf struct {
	collections []models.Collection
	articles    []models.ListPublishedArticlesRow
	originals   map[ids.UUID]bool
}

/*
in returns what readers of the language see. Unless untranslated
articles are hidden, an article not translated into the language is
shown in the default one, and a collection holding one shows under its
own name when it has no translated one.
*/
func (s shelves) in(locale, untranslated string) shelf {
	if locale == s.center.Locale {
		return shelf{collections: s.collections, articles: s.articles}
	}
	out := s.articlesIn(locale, untranslated)
	out.collections = s.collectionsIn(locale, untranslated, out.articles)
	return out
}

/*
articlesIn lists the articles readers of the language see, in the
source's order, then those translated whose source is not published.
*/
func (s shelves) articlesIn(locale, untranslated string) shelf {
	translated := map[ids.UUID]models.ListPublishedArticlesRow{}
	var order []ids.UUID
	for _, t := range s.translated {
		if t.Locale == locale {
			translated[t.ID] = articleRowOf(t)
			order = append(order, t.ID)
		}
	}
	out := shelf{originals: map[ids.UUID]bool{}}
	for _, a := range s.articles {
		t, ok := translated[a.ID]
		switch {
		case ok:
			out.articles = append(out.articles, t)
			delete(translated, a.ID)
		case untranslated != UntranslatedHide:
			out.articles, out.originals[a.ID] = append(out.articles, a), true
		}
	}
	for _, id := range order {
		if t, ok := translated[id]; ok {
			out.articles = append(out.articles, t)
		}
	}
	out.articles = refreshed(out.articles)
	return out
}

/*
collectionsIn lists the collections readers of the language see, with
their names in it where translated.
*/
func (s shelves) collectionsIn(locale, untranslated string, articles []models.ListPublishedArticlesRow,
) []models.Collection {
	named := map[ids.UUID]models.ListCollectionTranslationsRow{}
	for _, t := range s.translations {
		if t.Locale == locale {
			named[t.CollectionID] = t
		}
	}
	holding := s.holding(articles)
	var out []models.Collection
	for _, c := range s.collections {
		t, ok := named[c.ID]
		if !holding[c.ID] || (!ok && untranslated == UntranslatedHide) {
			continue
		}
		if ok {
			c = namedIn(c, t)
		}
		out = append(out, c)
	}
	return out
}

/*
namedIn writes a collection's translation over it.
*/
func namedIn(c models.Collection, t models.ListCollectionTranslationsRow) models.Collection {
	c.Name, c.Slug, c.Description = t.Name, t.Slug, t.Description
	if t.UpdatedAt.After(c.UpdatedAt) {
		c.UpdatedAt = t.UpdatedAt
	}
	return c
}

/*
holding marks the collections holding one of the articles, directly or
in a collection below them.
*/
func (s shelves) holding(articles []models.ListPublishedArticlesRow) map[ids.UUID]bool {
	parent := map[ids.UUID]*ids.UUID{}
	for _, c := range s.collections {
		parent[c.ID] = c.ParentID
	}
	out := map[ids.UUID]bool{}
	for _, a := range articles {
		for at := a.CollectionID; at != nil && !out[*at]; at = parent[*at] {
			out[*at] = true
		}
	}
	return out
}

/*
alternates gives every article and collection its slug in each language
it shows in.
*/
func (s shelves) alternates() Alternates {
	out := Alternates{Articles: map[ids.UUID]map[string]string{}, Collections: map[ids.UUID]map[string]string{}}
	put := func(m map[ids.UUID]map[string]string, id ids.UUID, locale, slug string) {
		if m[id] == nil {
			m[id] = map[string]string{}
		}
		m[id][locale] = slug
	}
	for _, a := range s.articles {
		put(out.Articles, a.ID, s.center.Locale, a.Slug)
	}
	for _, c := range s.collections {
		put(out.Collections, c.ID, s.center.Locale, c.Slug)
	}
	for _, locale := range LocalesOf(s.center)[1:] {
		read := s.in(locale, UntranslatedHide)
		for _, a := range read.articles {
			put(out.Articles, a.ID, locale, a.Slug)
		}
		for _, c := range read.collections {
			put(out.Collections, c.ID, locale, c.Slug)
		}
	}
	return out
}

/*
articleRowOf reads a published translation as the source's row.
*/
func articleRowOf(t models.ListPublishedArticleTranslationsRow) models.ListPublishedArticlesRow {
	return models.ListPublishedArticlesRow{
		ID: t.ID, CollectionID: t.CollectionID, Position: t.Position, PublishedAt: t.PublishedAt, Title: t.Title,
		Slug: t.Slug, Description: t.Description, BodyMarkdown: t.BodyMarkdown, BodyHtml: t.BodyHtml, Toc: t.Toc,
		VersionID: t.VersionID, VersionCreatedAt: t.VersionCreatedAt, RendererVersion: t.RendererVersion,
	}
}

/*
searchIn finds the published public articles matching the query in the
search's language.
*/
func searchIn(ctx context.Context, q *models.Queries, f Find) ([]models.SearchPublishedArticlesRow, error) {
	h := f.Center
	source := func() ([]models.SearchPublishedArticlesRow, error) {
		return q.SearchPublishedArticles(ctx, models.SearchPublishedArticlesParams{
			Query: f.Query, WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, IncludeAgentOnly: false, RowLimit: f.Limit,
		})
	}
	locale := readLocale(h, f.Locale)
	if locale == h.Locale {
		return source()
	}
	rows, err := q.SearchPublishedArticleTranslations(ctx, models.SearchPublishedArticleTranslationsParams{
		Query: f.Query, WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Locale: locale, RowLimit: f.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]models.SearchPublishedArticlesRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.SearchPublishedArticlesRow{
			ID: r.ID, CollectionID: r.CollectionID, Title: r.Title, Slug: r.Slug, Description: r.Description,
			BodyMarkdown: r.BodyMarkdown, Visibility: "public", Rank: r.Rank, Snippet: r.Snippet,
		})
	}
	if !SettingsOf(h).ShowsOriginals() {
		return out, nil
	}
	return withOriginals(ctx, q, searchedIn{search: f, locale: locale, rows: out}, source)
}

/*
searchedIn is what a search of one language found so far.
*/
type searchedIn struct {
	search Find
	locale string
	rows   []models.SearchPublishedArticlesRow
}

/*
withOriginals adds to a search of one language the articles found in the
default language that are not translated into it, after the translated
ones.
*/
func withOriginals(ctx context.Context, q *models.Queries, so searchedIn,
	source func() ([]models.SearchPublishedArticlesRow, error),
) ([]models.SearchPublishedArticlesRow, error) {
	f, locale, out := so.search, so.locale, so.rows
	translated, err := q.ListTranslatedArticleIDs(ctx, models.ListTranslatedArticleIDsParams{
		WorkspaceID: f.Center.WorkspaceID, HelpCenterID: f.Center.ID, Locale: locale,
	})
	if err != nil {
		return nil, err
	}
	rows, err := source()
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if len(out) < int(f.Limit) && !slices.Contains(translated, r.ID) {
			out = append(out, r)
		}
	}
	return out, nil
}
