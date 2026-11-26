/*
|--------------------------------------------------------------------------
| Theme Variables: Pages And Collections
|--------------------------------------------------------------------------
|
| What a page says about itself, and the collections and articles the
| help pages list.
|
*/

package helpcenter

import "strings"

/*
AlternateVars is a page's address in one language.
*/
type AlternateVars struct {
	// The language tag, or x-default.
	Lang string
	// The full address.
	URL string
}

/*
PageVars is page: what the page says about itself.
*/
type PageVars struct {
	// The template being rendered, such as article.
	Template string
	// The page's title, for the title tag.
	Title string
	// The line search engines show, empty for none.
	Description string
	// The key of the section the page belongs to.
	Section string
	// The trail from the section's start to the page.
	Trail []CrumbVars
	// The page's Markdown twin, empty for none.
	MarkdownURL string
	// Search engines should skip the page.
	NoIndex bool
	// The page lists something to browse in a drawer.
	HasDrawer bool
	// The drawer's title.
	DrawerTitle string
}

/*
CrumbVars is a step of a trail.
*/
type CrumbVars struct {
	// The step's title.
	Title string
	// Its address.
	URL string
}

/*
SummaryVars is an article as a list shows it.
*/
type SummaryVars struct {
	// Its slug.
	Slug string
	// Its title.
	Title string
	// Its summary.
	Description string
	// The title of its collection.
	Collection string
	// Its address.
	URL string
}

/*
CollectionVars is a collection with its articles and sub-collections.
*/
type CollectionVars struct {
	// Its slug.
	Slug string
	// Its name.
	Title string
	// Its description.
	Description string
	// Its icon's name.
	Icon string
	// How many articles it and its sub-collections hold.
	Count int
	// Its address.
	URL string
	// Its Markdown twin.
	MarkdownURL string
	// Who writes its articles.
	Authors []PersonVars
	// Who writes it, in words, such as By Ada and 2 others.
	Byline string
	// Its own articles.
	Articles []SummaryVars
	// Its sub-collections.
	Children []CollectionVars
	// Its slug and every sub-collection's, to test whether it holds a collection.
	Slugs []string
}

func (c Center) pageVars(name string, m Meta) PageVars {
	v := PageVars{
		Template: name, Title: m.Title, Description: m.Description, Section: m.Section, NoIndex: m.NoIndex || c.NoIndex,
		HasDrawer: m.Section == "help" || m.Section == "conversations", DrawerTitle: c.T("browse_articles"),
	}
	if m.Section == "conversations" {
		v.DrawerTitle = c.T("your_conversations")
	}
	if m.Markdown != "" {
		v.MarkdownURL = c.url(m.Markdown)
	}
	v.Trail = c.crumbs(m.Trail)
	return v
}

func (c Center) crumbs(trail []Crumb) []CrumbVars {
	out := make([]CrumbVars, 0, len(trail))
	for _, t := range trail {
		out = append(out, CrumbVars{Title: t.Title, URL: c.url(t.Path)})
	}
	return out
}

func (c Center) summary(s Summary) SummaryVars {
	return SummaryVars{
		Slug: s.Slug, Title: s.Title, Description: s.Description, Collection: s.Collection,
		URL: c.url("/articles/" + s.Slug),
	}
}

func (c Center) summaries(list []Summary) []SummaryVars {
	out := make([]SummaryVars, 0, len(list))
	for _, s := range list {
		out = append(out, c.summary(s))
	}
	return out
}

func (c Center) collection(col Collection) CollectionVars {
	v := CollectionVars{
		Slug: col.Slug, Title: col.Title, Description: col.Description, Icon: col.Icon, Count: col.Count,
		URL: c.url("/collections/" + col.Slug), MarkdownURL: c.url("/collections/" + col.Slug + ".md"),
		Authors: people(col.Authors), Byline: byline(c, col.Authors), Articles: c.summaries(col.Articles),
		Slugs: []string{col.Slug},
	}
	for _, sub := range col.Children {
		child := c.collection(sub)
		v.Children = append(v.Children, child)
		v.Slugs = append(v.Slugs, child.Slugs...)
	}
	return v
}

func (c Center) collections(all []Collection) []CollectionVars {
	out := make([]CollectionVars, 0, len(all))
	for _, col := range all {
		out = append(out, c.collection(col))
	}
	return out
}

/*
helpTrail is the trail to a help page: the help center, the collections
leading to it, and the page's own title.
*/
func helpTrail(c Center, trail []Crumb, collection, page string) []Crumb {
	out := append([]Crumb{{Title: c.T("help_center"), Path: "/"}}, trail...)
	if collection != "" {
		out = append(out, Crumb{Title: collection, Path: "/"})
	}
	if page != "" {
		out = append(out, Crumb{Title: page, Path: "/"})
	}
	return out
}

/*
collectionBySlug finds the collection with the slug at any depth.
*/
func collectionBySlug(all []Collection, slug string) (Collection, bool) {
	for _, col := range all {
		if col.Slug == slug {
			return col, true
		}
		if found, ok := collectionBySlug(col.Children, slug); ok {
			return found, true
		}
	}
	return Collection{}, false
}

func trailEnd(trail []Crumb) string {
	if len(trail) == 0 {
		return ""
	}
	return strings.TrimPrefix(trail[len(trail)-1].Path, "/collections/")
}

func byline(c Center, list []Person) string {
	switch len(list) {
	case 0:
		return c.T("by_team")
	case 1:
		return c.T("by_one", "name", list[0].Name)
	default:
		return c.T("by_many", "name", list[0].Name, "count", len(list)-1)
	}
}
