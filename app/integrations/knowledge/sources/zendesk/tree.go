/*
|--------------------------------------------------------------------------
| Zendesk Tree
|--------------------------------------------------------------------------
|
| The help center as Zendesk arranges it: categories, sections inside
| them and inside each other, and articles inside sections. Ticking a
| category or a section reads every article under it, including
| articles added to it later. Categories and sections are named with a
| prefix so their ids never meet an article's.
|
*/

package zendesk

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
maxPages bounds how many cursor pages one listing of categories,
sections or articles walks.
*/
const maxPages = 200

/*
Tree returns the categories, sections and articles as Zendesk nests
them, in the help center's default language.
*/
func (d *Driver) Tree(ctx context.Context, cfg sources.Config) ([]sources.Node, error) {
	s, err := read(cfg)
	if err != nil {
		return nil, err
	}

	categories, err := d.every(ctx, s, "categories")
	if err != nil {
		return nil, err
	}

	sections, err := d.every(ctx, s, "sections")
	if err != nil {
		return nil, err
	}

	articles, err := d.allArticles(ctx, s)
	if err != nil {
		return nil, err
	}

	under := map[int64][]sources.Node{}
	for _, a := range articles {
		under[a.SectionID] = append(under[a.SectionID], sources.Node{
			ID: strconv.FormatInt(a.ID, 10), Title: a.Title, Kind: "article", Children: []sources.Node{},
		})
	}

	out := []sources.Node{}
	for _, c := range categories {
		out = append(out, sources.Node{
			ID: categoryKey(c.ID), Title: c.Name, Kind: "collection",
			Children: branch(sections, under, c.ID, nil),
		})
	}

	return out, nil
}

/*
branch builds the sections of a category under a parent section, or at
the top of the category when parent is nil, each with its articles.
*/
func branch(sections []section, articles map[int64][]sources.Node, category int64, parent *int64) []sources.Node {
	out := []sources.Node{}

	for _, sec := range sections {
		if sec.CategoryID != category || !sameParent(sec.ParentSectionID, parent) {
			continue
		}

		id := sec.ID
		children := append(branch(sections, articles, category, &id), articles[sec.ID]...)
		out = append(out, sources.Node{ID: sectionKey(sec.ID), Title: sec.Name, Kind: "collection", Children: children})
	}

	return out
}

func sameParent(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

/*
chooser returns whether an article is read under the source's ticks.
It reads the sections only when something is ticked.
*/
func (d *Driver) chooser(ctx context.Context, s settings) (func(article) bool, error) {
	if len(s.Selected) == 0 {
		return func(article) bool { return true }, nil
	}

	list, err := d.every(ctx, s, "sections")
	if err != nil {
		return nil, err
	}

	sections := map[int64]section{}
	for _, sec := range list {
		sections[sec.ID] = sec
	}

	return func(a article) bool {
		return sources.Chosen(s.Selected, lineage(a, sections)...)
	}, nil
}

/*
lineage is an article and every section and category above it.
*/
func lineage(a article, sections map[int64]section) []string {
	out := []string{strconv.FormatInt(a.ID, 10)}

	next := &a.SectionID
	for i := 0; i <= maxSections && next != nil; i++ {
		sec, ok := sections[*next]
		if !ok {
			break
		}

		out = append(out, sectionKey(sec.ID), categoryKey(sec.CategoryID))
		next = sec.ParentSectionID
	}

	return out
}

func categoryKey(id int64) string {
	return "category:" + strconv.FormatInt(id, 10)
}

func sectionKey(id int64) string {
	return "section:" + strconv.FormatInt(id, 10)
}

/*
every reads all the categories or all the sections, walking Zendesk's
cursor pages.
*/
func (d *Driver) every(ctx context.Context, s settings, kind string) ([]section, error) {
	var out []section

	after := ""
	for range maxPages {
		address := fmt.Sprintf("%s/%s.json?page[size]=%d", s.base(), kind, pageSize)
		if after != "" {
			address += "&page[after]=" + url.QueryEscape(after)
		}

		var page struct {
			Categories []section `json:"categories"`
			Sections   []section `json:"sections"`
			Meta       struct {
				HasMore     bool   `json:"has_more"`
				AfterCursor string `json:"after_cursor"`
			} `json:"meta"`
		}

		if err := d.get(ctx, s, address, &page); err != nil {
			return nil, err
		}

		out = append(append(out, page.Categories...), page.Sections...)
		if !page.Meta.HasMore {
			break
		}

		after = page.Meta.AfterCursor
	}

	return out, nil
}

/*
allArticles reads every article in the help center's default language.
*/
func (d *Driver) allArticles(ctx context.Context, s settings) ([]article, error) {
	locales, err := d.locales(ctx, s)
	if err != nil {
		return nil, err
	}

	var out []article

	after := ""
	for range maxPages {
		page, err := d.articles(ctx, s, locales[0], after)
		if err != nil {
			return nil, err
		}

		out = append(out, page.Articles...)
		if !page.Meta.HasMore {
			break
		}

		after = page.Meta.AfterCursor
	}

	return out, nil
}
