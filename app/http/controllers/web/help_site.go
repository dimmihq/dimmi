/*
|--------------------------------------------------------------------------
| Help Center Site
|--------------------------------------------------------------------------
|
| A real help center shaped for the public pages: its collections with
| their articles, an article with its trail, contents and neighbors, and
| search. The pages read the same shapes as the sample help center, so
| one set of pages serves both.
|
*/

package web

import (
	"context"
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
wordsPerMinute is the reading speed the reading time assumes.
*/
const wordsPerMinute = 220

/*
HelpLibrary is what the help center's pages read: the sample help
center or a real one.
*/
type HelpLibrary interface {
	HelpShelves
	HelpReader
}

/*
HelpShelves is how a help center's articles are laid out for browsing.
*/
type HelpShelves interface {
	/*
		Collections returns the top-level collections with their articles
		and sub-collections.
	*/
	Collections() []helpcenter.Collection

	/*
		Popular returns the articles the home page features.
	*/
	Popular() []helpcenter.Summary

	/*
		Updates returns at most count recent changes worth announcing.
	*/
	Updates(count int) []helpcenter.Update

	/*
		Collection returns the collection with the slug and its trail, and
		false when there is none.
	*/
	Collection(slug string) (helpcenter.Collection, []helpcenter.Crumb, bool)
}

/*
HelpReader is how a reader opens and finds a help center's articles.
*/
type HelpReader interface {
	/*
		Article returns the article page with the slug inside the help
		center's frame, and false when there is none.
	*/
	Article(c helpcenter.Center, slug string) (helpcenter.Article, bool, error)

	/*
		Markdown returns the Markdown source of the article with the slug,
		and false when there is none.
	*/
	Markdown(slug string) (string, bool)

	/*
		Search returns the articles matching the query, best first.
	*/
	Search(query string) []helpcenter.Result
}

/*
site is a real help center as the pages read it, searching through the
reader bound to the request.
*/
type site struct {
	snap    helpcenters.Snapshot
	search  func(query string) []models.SearchPublishedArticlesRow
	updates []helpcenter.Update
	at      pageAt
}

/*
newSite shapes a snapshot for the pages, searching as the reader, nil for
anyone.
*/
func newSite(ctx context.Context, public *helpcenters.Public, snap helpcenters.Snapshot, reader *ids.UUID) *site {
	return &site{snap: snap, search: func(query string) []models.SearchPublishedArticlesRow {
		find := helpcenters.Find{Center: snap.HelpCenter, Locale: snap.Locale, Query: query, Limit: 20, Reader: reader}
		rows, err := public.Search(ctx, find)
		if err != nil {
			return nil
		}
		return rows
	}}
}

/*
centerOf is the page frame of a help center served under the base, with
the workspace's square logo as its mark, addressed on the app so every
host can load it.
*/
func (c *HelpCenterController) centerOf(snap helpcenters.Snapshot, base string) helpcenter.Center {
	h := snap.HelpCenter
	s := helpcenters.SettingsOf(h)
	brand := s.PrimaryColor
	if brand == "" {
		brand = "#10b981"
	}
	center := helpcenter.Center{
		Lang: c.Lang, Name: h.Name, Product: h.Name, Base: base, Brand: brand, Tagline: s.HomeSubtitle,
		ContactURL: s.ContactURL,
		Appearance: s.Appearance, HomeTitle: s.HomeTitle, FooterText: s.FooterText, NoIndex: !h.IndexingEnabled,
	}
	if snap.Logo != "" {
		center.Mark = strings.TrimSuffix(c.AppURL, "/") + "/" + snap.Logo
	}
	return center
}

/*
Collections returns the top-level collections with their articles.
*/
func (s *site) Collections() []helpcenter.Collection {
	var out []helpcenter.Collection
	for _, c := range s.snap.Collections {
		if c.ParentID == nil {
			out = append(out, s.view(c))
		}
	}
	return out
}

/*
Popular returns the first published articles, in collection order.
*/
func (s *site) Popular() []helpcenter.Summary {
	var out []helpcenter.Summary
	for i, a := range s.snap.Articles {
		if i == 6 {
			break
		}
		out = append(out, s.summary(a))
	}
	return out
}

/*
Updates returns the newest changelog entries, for the home page.
*/
func (s *site) Updates(n int) []helpcenter.Update { return s.updates[:min(n, len(s.updates))] }

/*
Collection returns the collection with the slug and its trail.
*/
func (s *site) Collection(slug string) (helpcenter.Collection, []helpcenter.Crumb, bool) {
	for _, c := range s.snap.Collections {
		if c.Slug == slug {
			return s.view(c), s.trail(c.ParentID), true
		}
	}
	return helpcenter.Collection{}, nil, false
}

/*
Article returns the published article with the slug, its trail,
contents and neighbors in its collection.
*/
func (s *site) Article(_ helpcenter.Center, slug string) (helpcenter.Article, bool, error) {
	at := s.find(slug)
	if at < 0 {
		return helpcenter.Article{}, false, nil
	}
	a := s.snap.Articles[at]
	page := helpcenter.Article{
		Summary: s.summary(a), Trail: s.trail(a.CollectionID), Body: a.BodyHtml, Contents: contents(a.Toc),
		Author: authorOf(a), Updated: a.VersionCreatedAt,
		Minutes: max(1, len(strings.Fields(a.BodyMarkdown))/wordsPerMinute),
	}
	if s.snap.Originals[a.ID] {
		page.Untranslated = helpcenters.LanguageOf(s.snap.Locale).Native
		page.OriginalLang = s.snap.HelpCenter.Locale
	}
	siblings := s.in(a.CollectionID)
	i := indexOf(siblings, a.ID)
	if i > 0 {
		page.Previous = new(s.summary(siblings[i-1]))
	}
	if i >= 0 && i+1 < len(siblings) {
		page.Next = new(s.summary(siblings[i+1]))
	}
	return page, true, nil
}

/*
Markdown returns the published Markdown of the article with the slug.
*/
func (s *site) Markdown(slug string) (string, bool) {
	at := s.find(slug)
	if at < 0 {
		return "", false
	}
	return s.snap.Articles[at].BodyMarkdown, true
}

/*
Search returns the published articles matching the query, with their
snippets as plain text.
*/
func (s *site) Search(query string) []helpcenter.Result {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	rows := s.search(query)
	out := make([]helpcenter.Result, 0, len(rows))
	for _, r := range rows {
		out = append(out, helpcenter.Result{
			Kind: "article", Title: r.Title, Path: "/articles/" + r.Slug, Excerpt: plain(r.Snippet),
			Trail: s.collectionTitle(r.CollectionID),
		})
	}
	return out
}

func (s *site) view(c models.Collection) helpcenter.Collection {
	v := helpcenter.Collection{Slug: c.Slug, Title: c.Name, Description: deref(c.Description), Icon: deref(c.Icon)}
	seen := map[string]bool{}
	credit := func(p helpcenter.Person) {
		if p.Name != "" && !seen[p.Name] {
			seen[p.Name] = true
			v.Authors = append(v.Authors, p)
		}
	}
	for _, a := range s.in(&c.ID) {
		v.Articles = append(v.Articles, s.summary(a))
		credit(authorOf(a))
	}
	for _, child := range s.snap.Collections {
		if child.ParentID != nil && *child.ParentID == c.ID {
			sub := s.view(child)
			v.Children = append(v.Children, sub)
			for _, p := range sub.Authors {
				credit(p)
			}
		}
	}
	v.Count = len(v.Articles)
	for _, child := range v.Children {
		v.Count += child.Count
	}
	return v
}

func (s *site) trail(collection *ids.UUID) []helpcenter.Crumb {
	if collection == nil {
		return nil
	}
	for _, c := range s.snap.Collections {
		if c.ID == *collection {
			return append(s.trail(c.ParentID), helpcenter.Crumb{Title: c.Name, Path: "/collections/" + c.Slug})
		}
	}
	return nil
}

func (s *site) in(collection *ids.UUID) []models.ListPublishedArticlesRow {
	var out []models.ListPublishedArticlesRow
	for _, a := range s.snap.Articles {
		if a.CollectionID != nil && collection != nil && *a.CollectionID == *collection {
			out = append(out, a)
		}
	}
	return out
}

func (s *site) summary(a models.ListPublishedArticlesRow) helpcenter.Summary {
	return helpcenter.Summary{
		Slug: a.Slug, Title: a.Title, Description: deref(a.Description), Collection: s.collectionTitle(a.CollectionID),
	}
}

func (s *site) collectionTitle(id *ids.UUID) string {
	if id == nil {
		return ""
	}
	for _, c := range s.snap.Collections {
		if c.ID == *id {
			return c.Name
		}
	}
	return ""
}

func (s *site) find(slug string) int {
	for i, a := range s.snap.Articles {
		if a.Slug == slug {
			return i
		}
	}
	return -1
}

/*
articleIn gives the path of the article with the slug in each language
it is published in.
*/
func (s *site) articleIn(slug string) pageIn {
	at := s.find(slug)
	if at < 0 {
		return func(string) (string, bool) { return "", false }
	}
	slugs := s.snap.Alternates.Article(s.snap.Articles[at].ID)
	return func(locale string) (string, bool) {
		other, ok := slugs[locale]
		return "/articles/" + other, ok
	}
}

/*
collectionIn gives the path of the collection with the slug in each
language it shows in.
*/
func (s *site) collectionIn(slug string) pageIn {
	for _, c := range s.snap.Collections {
		if c.Slug == slug {
			slugs := s.snap.Alternates.Collection(c.ID)
			return func(locale string) (string, bool) {
				other, ok := slugs[locale]
				return "/collections/" + other, ok
			}
		}
	}
	return func(string) (string, bool) { return "", false }
}

func indexOf(rows []models.ListPublishedArticlesRow, id ids.UUID) int {
	for i, a := range rows {
		if a.ID == id {
			return i
		}
	}
	return -1
}

/*
contents reads a version's stored table of contents into the page's
nested headings.
*/
func contents(raw json.RawMessage) []helpcenter.Heading {
	var toc []helpcenters.TOCEntry
	_ = json.Unmarshal(raw, &toc)
	var out []helpcenter.Heading
	for _, e := range toc {
		h := helpcenter.Heading{Title: e.Text, ID: e.Anchor}
		if e.Level == 3 && len(out) > 0 {
			out[len(out)-1].Children = append(out[len(out)-1].Children, h)
			continue
		}
		out = append(out, h)
	}
	return out
}

/*
plain turns a search snippet into the words a reader sees. It drops the
highlight marks the database adds, keeps the text of links and drops
images, and removes the Markdown punctuation for emphasis, headings,
quotes, callouts and code, since the snippet is cut from the article's
Markdown source. A snippet cut from the middle of a sentence opens with
an ellipsis.
*/
func plain(snippet string) string {
	text := regexp.MustCompile(`</?mark>`).ReplaceAllString(snippet, "")
	text = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\[![A-Za-z]+\]|(^|\s)(#{1,6}|>|[-*+]|\d+\.)\s`).ReplaceAllString(text, " ")
	text = strings.NewReplacer("**", "", "__", "", "`", "", "~~", "").Replace(text)
	text = strings.Join(strings.Fields(html.UnescapeString(text)), " ")
	if first, _ := utf8.DecodeRuneInString(text); unicode.IsLower(first) {
		return "…" + text
	}
	return text
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

/*
authorOf is the teammate who wrote an article, with their photo when
they have one, and no name when no teammate did.
*/
func authorOf(a models.ListPublishedArticlesRow) helpcenter.Person {
	p := helpcenter.Person{Name: a.AuthorName, Initials: initialsOf(a.AuthorName)}
	if photo := (models.User{AvatarPath: a.AuthorAvatar}).AvatarURL(); photo != nil {
		p.Photo = *photo
	}
	return p
}
