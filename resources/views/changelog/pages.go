/*
|--------------------------------------------------------------------------
| Changelog Pages
|--------------------------------------------------------------------------
|
| The changelog's pages in the help center's theme: the timeline, one
| entry, and the page a subscriber's link opens.
|
*/

package changelog

import (
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/liquid"
	"github.com/dimmihq/dimmi/framework/view"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
LabelVars is a kind of change an entry is filed under.
*/
type LabelVars struct {
	// Its name, such as New or Fixed.
	Name string
	// Its slug.
	Slug string
	// The hex color of its dot.
	Color string
	// Its own filtered page.
	URL string
	// How many entries carry it.
	Count int
}

/*
ReactionVars is an emoji readers react with.
*/
type ReactionVars struct {
	// The emoji.
	Emoji string
	// The name it is stored under, such as thumbsup.
	Name string
	// How many reacted with it.
	Count int
}

/*
EntryVars is a changelog entry.
*/
type EntryVars struct {
	// Its slug.
	Slug string
	// Its title.
	Title string
	// Its summary.
	Summary string
	// Its body, sanitized HTML.
	Body liquid.HTML
	// Its labels.
	Labels []LabelVars
	// Its version, empty for none.
	Version string
	// Its cover image's address, empty for none.
	Cover string
	// When it was published.
	Published time.Time
	// Who wrote it.
	Authors []helpcenter.PersonVars
	// Their names joined with &.
	AuthorNames string
	// The emoji readers may react with, none when reactions are off.
	Reactions []ReactionVars
	// Where the reaction form posts, with emoji.
	ReactionsURL string
	// Articles that explain it.
	Related []helpcenter.SummaryVars
	// A list shows its whole body rather than its summary.
	Full bool
	// Its page.
	URL string
}

/*
WeekVars is a week of the release strip.
*/
type WeekVars struct {
	// The month it opens, when it is the first week of one, else empty.
	MonthName string
	// The entry published in it, empty for none.
	Title string
	// That entry's page, empty for none.
	URL string
}

/*
LogVars is changelog, on the changelog's pages.
*/
type LogVars struct {
	// The changelog's name.
	Title string
	// The changelog takes no subscriptions.
	NoSubscribe bool
	// The reader just asked to subscribe.
	Subscribed bool
	// Its RSS feed, narrowed to the label shown.
	FeedURL string
	// This page's entries, newest first.
	Entries []EntryVars
	// Every label to filter by.
	Labels []LabelVars
	// All updates, then each label, as chips.
	Chips []helpcenter.LinkVars
	// The slug of the label shown, empty for all.
	Active string
	// The page of newer entries, empty for none.
	NewerURL string
	// The page of older entries, empty for none.
	OlderURL string
	// The year the release strip shows.
	YearText string
	// The year's weeks.
	Weeks []WeekVars
}

func labels(c helpcenter.Center, list []Label) []LabelVars {
	out := make([]LabelVars, 0, len(list))
	for _, l := range list {
		out = append(
			out,
			LabelVars{Name: l.Name, Slug: l.Slug, Color: l.Color, URL: helpcenter.Href(c, l.Path), Count: l.Count},
		)
	}
	return out
}

func entry(c helpcenter.Center, e Entry) EntryVars {
	v := EntryVars{
		Slug: e.Slug, Title: e.Title, Summary: e.Summary, Body: liquid.HTML(e.Body), Labels: labels(
			c, e.Labels,
		), Version: e.Version,
		Published: e.Published, AuthorNames: authorNames(e.Authors), Full: e.Full, URL: helpcenter.Href(c, e.Path),
		ReactionsURL: helpcenter.Href(c, e.Path+"/reactions"),
	}
	if e.Cover != "" {
		v.Cover = helpcenter.Path(c, e.Cover)
	}
	for _, p := range e.Authors {
		v.Authors = append(
			v.Authors,
			helpcenter.PersonVars{Name: p.Name, Initials: p.Initials, Role: p.Role, Photo: p.Photo},
		)
	}
	for _, r := range e.Reactions {
		v.Reactions = append(v.Reactions, ReactionVars(r))
	}
	for _, a := range e.Related {
		v.Related = append(v.Related, helpcenter.SummaryVars{
			Slug: a.Slug, Title: a.Title, Description: a.Description,
			Collection: a.Collection, URL: helpcenter.Href(c, "/articles/"+a.Slug),
		})
	}
	return v
}

/*
IndexPage is the changelog's timeline.
*/
func IndexPage(c helpcenter.Center, index Index) view.Component {
	m := helpcenter.Meta{
		Title: indexTitle(c, index), Description: c.T("changelog_description", "product", c.Product),
		Section: "changelog", Trail: indexTrail(c, index), Markdown: "/changelog.md",
	}
	v := LogVars{
		Title: c.ChangelogTitle(index.Title), NoSubscribe: index.NoSubscribe, Subscribed: index.Subscribed,
		FeedURL: helpcenter.Href(c, feedPath(index.Active)), Labels: labels(c, index.Labels), Active: index.Active,
		YearText: strconv.Itoa(index.Year),
	}
	if index.Newer != "" {
		v.NewerURL = helpcenter.Href(c, index.Newer)
	}
	if index.Older != "" {
		v.OlderURL = helpcenter.Href(c, index.Older)
	}
	for _, e := range index.Entries {
		v.Entries = append(v.Entries, entry(c, e))
	}
	for _, w := range index.Weeks {
		week := WeekVars{MonthName: c.Month(w.Month), Title: w.Title}
		if w.Path != "" {
			week.URL = helpcenter.Href(c, w.Path)
		}
		v.Weeks = append(v.Weeks, week)
	}
	v.Chips = append(
		v.Chips,
		helpcenter.LinkVars{
			Name: c.T("all_updates"), URL: helpcenter.Href(c, "/changelog"), Current: index.Active == "",
		},
	)
	for _, l := range index.Labels {
		v.Chips = append(
			v.Chips,
			helpcenter.LinkVars{
				Name: l.Name, URL: helpcenter.Href(c, l.Path), Color: l.Color, Count: l.Count,
				Current: index.Active == l.Slug,
			},
		)
	}
	return helpcenter.Render(c, "changelog", m, map[string]any{"changelog": v})
}

/*
EntryPage shows one entry in full with the entries around it.
*/
func EntryPage(c helpcenter.Center, _ []Label, p Page) view.Component {
	m := helpcenter.Meta{
		Title: c.T("changelog_entry_title", "title", p.Entry.Title, "product", c.Product), Description: p.Entry.Summary,
		Section: "changelog", Trail: []helpcenter.Crumb{
			{Title: c.ChangelogTitle(p.Title), Path: "/changelog"},
			{Title: p.Entry.Title, Path: p.Entry.Path},
		},
		Markdown: p.Entry.Path + ".md",
	}
	vars := map[string]any{
		"entry":     entry(c, p.Entry),
		"changelog": LogVars{Title: c.ChangelogTitle(p.Title), NoSubscribe: p.NoSubscribe},
	}
	if p.Previous != nil {
		vars["previous"] = entry(c, *p.Previous)
	}
	if p.Next != nil {
		vars["next"] = entry(c, *p.Next)
	}
	return helpcenter.Render(c, "changelog_entry", m, vars)
}

func authorNames(people []helpcenter.Person) string {
	names := make([]string, 0, len(people))
	for _, p := range people {
		names = append(names, p.Name)
	}
	return strings.Join(names, " & ")
}

func feedPath(label string) string {
	if label == "" {
		return "/changelog/feed.xml"
	}
	return "/changelog/labels/" + label + "/feed.xml"
}

func indexTrail(c helpcenter.Center, index Index) []helpcenter.Crumb {
	trail := []helpcenter.Crumb{{Title: c.ChangelogTitle(index.Title), Path: "/changelog"}}
	for _, l := range index.Labels {
		if l.Slug == index.Active {
			trail = append(trail, helpcenter.Crumb{Title: l.Name, Path: l.Path})
		}
	}
	return trail
}

func indexTitle(c helpcenter.Center, index Index) string {
	title := c.ChangelogTitle(index.Title) + " | " + c.Product
	for _, l := range index.Labels {
		if l.Slug == index.Active {
			return l.Name + " · " + title
		}
	}
	return title
}
