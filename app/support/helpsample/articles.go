/*
|--------------------------------------------------------------------------
| Sample Articles
|--------------------------------------------------------------------------
|
| The help center's pages built from the sample articles: the home page's
| collections and most read articles, a collection with its
| sub-collections, an article with its neighbors, and search. Search
| matches every word of the query against titles, descriptions and
| bodies, the way the real keyword search reads to a visitor.
|
*/

package helpsample

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
article is a sample article as read from its file.
*/
type article struct {
	slug        string
	title       string
	description string
	collection  string
	author      string
	updated     time.Time
	body        string
}

/*
wordsPerMinute is the reading speed article lengths are estimated at.
*/
const wordsPerMinute = 220

func readArticles() ([]article, error) {
	var out []article

	err := readDir("articles", func(slug string, meta map[string]any, body string) error {
		updated, err := date(meta["updated"])
		if err != nil {
			return err
		}

		out = append(out, article{
			slug: slug, title: text(meta["title"]), description: text(meta["description"]),
			collection: text(meta["collection"]), author: text(meta["author"]), updated: updated, body: body,
		})

		return nil
	})

	return out, err
}

/*
Collections returns the top-level collections with their
sub-collections and articles, as the home page and the sidebar list
them.
*/
func (l *Library) Collections() []helpcenter.Collection {
	var out []helpcenter.Collection

	for _, c := range l.collections {
		if c.parent == "" {
			out = append(out, l.view(c))
		}
	}

	return out
}

/*
Popular returns the articles the home page features.
*/
func (l *Library) Popular() []helpcenter.Summary {
	var out []helpcenter.Summary

	for _, slug := range []string{
		"install-on-nextjs", "identify-your-users", "connect-your-support-email",
		"how-dimmi-agent-answers", "use-a-custom-domain", "plans-and-pricing",
	} {
		if a, ok := l.findArticle(slug); ok {
			out = append(out, l.summary(a))
		}
	}

	return out
}

/*
Collection returns the collection with the slug and the trail to it.
*/
func (l *Library) Collection(slug string) (helpcenter.Collection, []helpcenter.Crumb, bool) {
	c, ok := l.findCollection(slug)
	if !ok {
		return helpcenter.Collection{}, nil, false
	}

	return l.view(c), l.trail(c.parent), true
}

/*
Article returns the article with the slug, rendered for the help center.
*/
func (l *Library) Article(c helpcenter.Center, slug string) (helpcenter.Article, bool, error) {
	a, ok := l.findArticle(slug)
	if !ok {
		return helpcenter.Article{}, false, nil
	}

	res, err := l.render(c, a.body)
	if err != nil {
		return helpcenter.Article{}, false, fmt.Errorf("render article %s: %w", slug, err)
	}

	col, _ := l.findCollection(a.collection)
	author, _ := person(a.author)
	siblings := l.in(a.collection)
	at := slices.IndexFunc(siblings, func(s article) bool { return s.slug == slug })

	page := helpcenter.Article{
		Summary: l.summary(a), Trail: append(l.trail(col.parent), helpcenter.Crumb{
			Title: col.title, Path: "/collections/" + col.slug,
		}),
		Body: res.HTML, Contents: headings(res.TOC), Author: author, Updated: a.updated,
		Minutes: max(1, len(strings.Fields(a.body))/wordsPerMinute),
	}

	if at > 0 {
		previous := l.summary(siblings[at-1])
		page.Previous = &previous
	}

	if at+1 < len(siblings) {
		next := l.summary(siblings[at+1])
		page.Next = &next
	}

	return page, true, nil
}

/*
Markdown returns the Markdown source of the article with the slug.
*/
func (l *Library) Markdown(slug string) (string, bool) {
	a, ok := l.findArticle(slug)

	return a.body, ok
}

/*
Search returns the articles matching every word of the query, title
matches first.
*/
func (l *Library) Search(query string) []helpcenter.Result {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil
	}

	type hit struct {
		result helpcenter.Result
		score  int
	}

	var hits []hit

	for _, a := range l.articles {
		haystack := strings.ToLower(a.title + " " + a.description + " " + a.body)
		if !containsAll(haystack, words) {
			continue
		}

		col, _ := l.findCollection(a.collection)
		hits = append(hits, hit{
			result: helpcenter.Result{
				Kind: "article", Title: a.title, Path: "/articles/" + a.slug,
				Excerpt: excerpt(a, words), Trail: col.title,
			},
			score: score(strings.ToLower(a.title), words),
		})
	}

	slices.SortStableFunc(hits, func(x, y hit) int { return cmp.Compare(y.score, x.score) })

	out := make([]helpcenter.Result, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.result)
	}

	return out
}

func (l *Library) view(c collection) helpcenter.Collection {
	v := helpcenter.Collection{Slug: c.slug, Title: c.title, Description: c.description, Icon: c.icon}
	seen := map[string]bool{}

	for _, a := range l.in(c.slug) {
		v.Articles = append(v.Articles, l.summary(a))
		seen = addAuthor(&v, seen, a.author)
	}

	for _, child := range l.collections {
		if child.parent != c.slug {
			continue
		}

		sub := l.view(child)
		v.Children = append(v.Children, sub)

		for _, p := range sub.Authors {
			if !seen[p.Name] {
				seen[p.Name] = true
				v.Authors = append(v.Authors, p)
			}
		}
	}

	v.Count = len(v.Articles)
	for _, child := range v.Children {
		v.Count += child.Count
	}

	return v
}

func addAuthor(v *helpcenter.Collection, seen map[string]bool, key string) map[string]bool {
	p, ok := person(key)
	if ok && !seen[p.Name] {
		seen[p.Name] = true
		v.Authors = append(v.Authors, p)
	}

	return seen
}

/*
trail returns the crumbs from the home page down to the collection with
the slug, empty for none.
*/
func (l *Library) trail(slug string) []helpcenter.Crumb {
	c, ok := l.findCollection(slug)
	if !ok {
		return nil
	}

	return append(l.trail(c.parent), helpcenter.Crumb{Title: c.title, Path: "/collections/" + c.slug})
}

func (l *Library) in(slug string) []article {
	var out []article

	for _, a := range l.articles {
		if a.collection == slug {
			out = append(out, a)
		}
	}

	return out
}

func (l *Library) summary(a article) helpcenter.Summary {
	col, _ := l.findCollection(a.collection)

	return helpcenter.Summary{Slug: a.slug, Title: a.title, Description: a.description, Collection: col.title}
}

func (l *Library) findCollection(slug string) (collection, bool) {
	for _, c := range l.collections {
		if c.slug == slug {
			return c, true
		}
	}

	return collection{}, false
}

func (l *Library) findArticle(slug string) (article, bool) {
	for _, a := range l.articles {
		if a.slug == slug {
			return a, true
		}
	}

	return article{}, false
}

func headings(items []markdown.TOCItem) []helpcenter.Heading {
	out := make([]helpcenter.Heading, 0, len(items))

	for _, item := range items {
		if item.ID == "" {
			out = append(out, headings(item.Items)...)

			continue
		}

		out = append(out, helpcenter.Heading{Title: item.Title, ID: item.ID, Children: headings(item.Items)})
	}

	return out
}

func containsAll(haystack string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(haystack, w) {
			return false
		}
	}

	return true
}

func score(title string, words []string) int {
	n := 0

	for _, w := range words {
		if strings.Contains(title, w) {
			n++
		}
	}

	return n
}

/*
excerptRunes is how much of the text around the first match a search
result shows.
*/
const excerptRunes = 160

/*
excerpt returns the description when it mentions a word of the query,
else the stretch of the body around the first match, cut at words.
*/
func excerpt(a article, words []string) string {
	if score(strings.ToLower(a.description), words) > 0 {
		return a.description
	}

	plain := strings.Join(strings.Fields(strings.NewReplacer("#", "", "*", "", "`", "", ">", "").Replace(a.body)), " ")
	at := strings.Index(strings.ToLower(plain), words[0])
	start := strings.LastIndex(plain[:max(0, at-excerptRunes/3)], " ") + 1
	cut := plain[start:]

	if utf8.RuneCountInString(cut) <= excerptRunes {
		return cut
	}

	cut = string([]rune(cut)[:excerptRunes])

	return strings.TrimRight(cut[:max(strings.LastIndex(cut, " "), 0)], ",.;:") + "…"
}
