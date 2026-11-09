/*
|--------------------------------------------------------------------------
| Real Changelog Pages
|--------------------------------------------------------------------------
|
| A real help center's changelog: its published entries shaped for the
| changelog's pages, the release strip of the year's weeks, and the
| neighbors of an entry. Only what is published ever reaches here.
|
*/

package web

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
	views "github.com/dimmihq/dimmi/resources/views/changelog"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
Entries a changelog page lists, and how many of the first page show
their whole body.
*/
const (
	changelogPerPage = 10
	changelogFull    = 3
)

/*
emojiOf returns each reaction's character and name, by its stored name.
*/
func emojiOf() map[string]views.Reaction {
	return map[string]views.Reaction{
		"thumbsup": {Emoji: "👍", Name: "Thumbs up"},
		"heart":    {Emoji: "❤️", Name: "Love"},
		"tada":     {Emoji: "🎉", Name: "Celebrate"},
		"rocket":   {Emoji: "🚀", Name: "Rocket"},
		"eyes":     {Emoji: "👀", Name: "Eyes"},
	}
}

/*
realChangelog is a real help center's changelog as a request reaches it:
the page frame, the help center, and the reader when one is signed in.
*/
type realChangelog struct {
	center helpcenter.Center
	help   models.HelpCenter
	reader *ids.UUID
}

/*
live finds the real help center the request names. Live is false when
none answers, so the sample serves the page; ok is false when the
request was already answered.
*/
func (c ChangelogController) live(w fhttp.ResponseWriter, r *fhttp.Request) (out realChangelog, live, ok bool) {
	if c.Public == nil || c.Changelog == nil {
		return out, false, true
	}
	snap, err := c.Public.Load(r.Context(), r.PathValue("center"))
	if errors.Is(err, helpcenters.ErrNoHelpCenter) {
		return out, false, true
	}
	if err != nil {
		c.Errors.Render(w, r, err)
		return out, true, false
	}
	base := "/hc/" + r.PathValue("center")
	if hostedHere(r.Context()) {
		base = ""
	}
	center, _, shown := c.realSite(w, r, snap, base)
	out = realChangelog{center: center, help: snap.HelpCenter}
	if contact, _, signed := c.readerOf(r, snap.HelpCenter); signed {
		out.reader = &contact
	}
	return out, true, shown
}

/*
indexView shapes a page of a real changelog for its page.
*/
func indexView(p changelog.Page, label string, page int) views.Index {
	out := views.Index{Active: label, Year: p.Year, Labels: labelViews(p.Labels)}
	for i, e := range p.Entries {
		if page == 1 && i < changelogFull {
			out.Entries = append(out.Entries, fullEntryView(e))
			continue
		}
		out.Entries = append(out.Entries, entryView(e))
	}
	base := "/changelog"
	if label != "" {
		base += "/labels/" + label
	}
	if page > 1 {
		out.Newer = pageOf(base, page-1)
	}
	if p.More {
		out.Older = pageOf(base, page+1)
	}
	out.Weeks = weeksOf(p.Year, p.Dates)
	return out
}

func pageOf(base string, page int) string {
	if page == 1 {
		return base
	}
	return base + "/page/" + strconv.Itoa(page)
}

func labelViews(in []changelog.LabelCount) []views.Label {
	out := make([]views.Label, 0, len(in))
	for _, l := range in {
		out = append(out, views.Label{
			Name: l.Name, Slug: l.Slug, Color: l.Color, Path: "/changelog/labels/" + l.Slug, Count: int(l.Entries),
		})
	}
	return out
}

/*
fullEntryView shapes a published entry with its body.
*/
func fullEntryView(e changelog.Published) views.Entry {
	out := entryView(e)
	out.Body, out.Full = deref(e.PublishedBodyHtml), true
	return out
}

/*
entryView shapes a published entry as lists show it, without its body.
*/
func entryView(e changelog.Published) views.Entry {
	out := views.Entry{
		Slug: e.Slug, Path: "/changelog/" + e.Slug, Title: deref(e.PublishedTitle),
		Summary: deref(e.PublishedSummary), Cover: deref(e.PublishedCoverUrl), Published: *e.PublishedAt,
	}
	for _, l := range e.Labels {
		out.Labels = append(out.Labels, views.Label{
			Name: l.Name, Slug: l.Slug, Color: l.Color, Path: "/changelog/labels/" + l.Slug,
		})
	}
	for _, a := range e.Authors {
		out.Authors = append(out.Authors, helpcenter.Person{Name: a.Name, Initials: initialsOf(a.Name)})
	}
	byName := emojiOf()
	for _, name := range changelog.Emoji() {
		r := byName[name]
		r.Count = int(e.Reactions[name])
		out.Reactions = append(out.Reactions, r)
	}
	return out
}

/*
weeksOf draws the year's release strip: every week, the month each
opens, and the newest entry shipped in it.
*/
func weeksOf(year int, dates []models.ListChangelogPublishDatesRow) []views.Week {
	const weeksInStrip = 52
	weeks := make([]views.Week, weeksInStrip)
	for month := time.January; month <= time.December; month++ {
		first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		weeks[min((first.YearDay()-1)/7, weeksInStrip-1)].Month = first.Format("Jan")
	}
	for _, d := range dates {
		w := &weeks[min((d.PublishedAt.UTC().YearDay()-1)/7, weeksInStrip-1)]
		if w.Path == "" {
			w.Title, w.Path = deref(d.PublishedTitle), "/changelog/"+d.Slug
		}
	}
	return weeks
}

/*
singleView shapes one published entry with its neighbors.
*/
func singleView(s changelog.Single) views.Page {
	out := views.Page{Entry: fullEntryView(s.Published)}
	if s.Newer != nil {
		out.Next = &views.Entry{
			Slug: s.Newer.Slug, Path: "/changelog/" + s.Newer.Slug, Title: s.Newer.Title, Published: s.Newer.Published,
		}
	}
	if s.Older != nil {
		out.Previous = &views.Entry{
			Slug: s.Older.Slug, Path: "/changelog/" + s.Older.Slug, Title: s.Older.Title, Published: s.Older.Published,
		}
	}
	return out
}

/*
initialsOf is the first letters of a name's first two words.
*/
func initialsOf(name string) string {
	var out []rune
	for word := range strings.FieldsSeq(name) {
		first, _ := utf8.DecodeRuneInString(strings.ToUpper(word))
		out = append(out, first)
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}
