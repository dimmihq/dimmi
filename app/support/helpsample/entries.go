/*
|--------------------------------------------------------------------------
| Sample Changelog
|--------------------------------------------------------------------------
|
| The changelog's pages built from the sample entries, newest first, as
| the real changelog pages them: the first entries of a list show in
| full and the rest as their summaries, so a page stays light however
| long the entries are.
|
*/

package helpsample

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/resources/views/changelog"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
entry is a sample changelog entry as read from its file.
*/
type entry struct {
	slug      string
	title     string
	summary   string
	labels    []string
	version   string
	published time.Time
	cover     string
	authors   []string
	reactions map[string]int
	related   []string
	body      string
}

/*
PerPage is how many entries a page of the changelog lists, and FullEntries
how many of them show their whole body.
*/
const (
	PerPage     = 5
	FullEntries = 3
)

func readEntries() ([]entry, error) {
	var out []entry

	err := readDir("changelog", func(slug string, meta map[string]any, body string) error {
		published, err := date(meta["published"])
		if err != nil {
			return err
		}

		out = append(out, entry{
			slug: slug, title: text(meta["title"]), summary: text(meta["summary"]),
			labels: texts(meta["labels"]), version: text(meta["version"]), published: published,
			cover: text(meta["cover"]), authors: texts(meta["authors"]), reactions: counts(meta["reactions"]),
			related: texts(meta["related"]), body: body,
		})

		return nil
	})

	slices.SortStableFunc(out, func(a, b entry) int { return b.published.Compare(a.published) })

	return out, err
}

/*
Changelog returns a page of entries, filtered to a label unless it is
empty, with the help center's paths. A page past the last one, or an
unknown label, is not found.
*/
func (l *Library) Changelog(c helpcenter.Center, labelSlug string, page int) (changelog.Index, bool, error) {
	base := "/changelog"
	if labelSlug != "" {
		if _, ok := label(labelSlug); !ok {
			return changelog.Index{}, false, nil
		}

		base += "/labels/" + labelSlug
	}

	matching := l.filter(labelSlug)
	from := (page - 1) * PerPage

	if page < 1 || from >= len(matching) {
		return changelog.Index{}, false, nil
	}

	index := changelog.Index{Active: labelSlug, Labels: l.Labels()}
	index.Year, index.Weeks = l.strip(labelSlug)

	for i, e := range matching[from:min(from+PerPage, len(matching))] {
		if page > 1 || i >= FullEntries {
			index.Entries = append(index.Entries, l.entryView(e))

			continue
		}

		v, err := l.fullEntry(c, e)
		if err != nil {
			return changelog.Index{}, false, err
		}

		index.Entries = append(index.Entries, v)
	}

	if page > 1 {
		index.Newer = pagePath(base, page-1)
	}

	if from+PerPage < len(matching) {
		index.Older = pagePath(base, page+1)
	}

	return index, true, nil
}

/*
Entry returns the entry with the slug and its neighbors.
*/
func (l *Library) Entry(c helpcenter.Center, slug string) (changelog.Page, bool, error) {
	at := slices.IndexFunc(l.entries, func(e entry) bool { return e.slug == slug })
	if at < 0 {
		return changelog.Page{}, false, nil
	}

	v, err := l.fullEntry(c, l.entries[at])
	if err != nil {
		return changelog.Page{}, false, err
	}

	page := changelog.Page{Entry: v}

	if at > 0 {
		newer := l.entryView(l.entries[at-1])
		page.Next = &newer
	}

	if at+1 < len(l.entries) {
		older := l.entryView(l.entries[at+1])
		page.Previous = &older
	}

	return page, true, nil
}

/*
EntryMarkdown returns the Markdown source of the entry with the slug.
*/
func (l *Library) EntryMarkdown(slug string) (string, bool) {
	at := slices.IndexFunc(l.entries, func(e entry) bool { return e.slug == slug })
	if at < 0 {
		return "", false
	}

	return l.entries[at].body, true
}

/*
Updates returns the newest entries for the help center's home.
*/
func (l *Library) Updates(count int) []helpcenter.Update {
	var out []helpcenter.Update

	for _, e := range l.entries[:min(count, len(l.entries))] {
		name := ""
		if lb, ok := label(firstOf(e.labels)); ok {
			name = lb.Name
		}

		out = append(out, helpcenter.Update{
			Title: e.title, Path: "/changelog/" + e.slug, Label: name, Published: e.published,
		})
	}

	return out
}

func (l *Library) filter(labelSlug string) []entry {
	if labelSlug == "" {
		return l.entries
	}

	var out []entry

	for _, e := range l.entries {
		if slices.Contains(e.labels, labelSlug) {
			out = append(out, e)
		}
	}

	return out
}

/*
entryView returns an entry as a list shows it, with its summary and
without its body.
*/
func (l *Library) entryView(e entry) changelog.Entry {
	v := changelog.Entry{
		Slug: e.slug, Path: "/changelog/" + e.slug, Title: e.title, Summary: e.summary,
		Version: e.version, Published: e.published,
	}

	if e.cover != "" {
		v.Cover = "/assets/" + e.cover
	}

	for _, slug := range e.labels {
		if lb, ok := label(slug); ok {
			lb.Path = "/changelog/labels/" + lb.Slug
			v.Labels = append(v.Labels, lb)
		}
	}

	for _, key := range e.authors {
		if p, ok := person(key); ok {
			v.Authors = append(v.Authors, p)
		}
	}

	for _, r := range reactions() {
		r.Count = e.reactions[r.Emoji]
		v.Reactions = append(v.Reactions, r)
	}

	for _, slug := range e.related {
		if a, ok := l.findArticle(slug); ok {
			v.Related = append(v.Related, l.summary(a))
		}
	}

	return v
}

/*
fullEntry returns an entry with its body rendered for the help center.
*/
func (l *Library) fullEntry(c helpcenter.Center, e entry) (changelog.Entry, error) {
	v := l.entryView(e)

	res, err := l.render(c, e.body)
	if err != nil {
		return changelog.Entry{}, fmt.Errorf("render entry %s: %w", e.slug, err)
	}

	v.Body, v.Full = res.HTML, true

	return v, nil
}

/*
Labels returns the changelog's labels with their pages and how many
entries carry each.
*/
func (l *Library) Labels() []changelog.Label {
	out := labels()
	for i := range out {
		out[i].Path = "/changelog/labels/" + out[i].Slug
		out[i].Count = len(l.filter(out[i].Slug))
	}

	return out
}

/*
weeksInStrip is how many weeks the release strip shows: a whole year.
*/
const weeksInStrip = 52

/*
strip returns the year of the newest entry and its weeks, each marked
with the entry published in it, filtered to a label unless it is empty.
*/
func (l *Library) strip(labelSlug string) (int, []changelog.Week) {
	if len(l.entries) == 0 {
		return 0, nil
	}

	year := l.entries[0].published.Year()
	weeks := make([]changelog.Week, weeksInStrip)

	for month := time.January; month <= time.December; month++ {
		first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		weeks[min((first.YearDay()-1)/7, weeksInStrip-1)].Month = first.Format("Jan")
	}

	for _, e := range l.filter(labelSlug) {
		if e.published.Year() != year {
			continue
		}

		w := &weeks[min((e.published.YearDay()-1)/7, weeksInStrip-1)]
		if w.Path == "" {
			w.Title, w.Path = e.title, "/changelog/"+e.slug
		}
	}

	return year, weeks
}

func pagePath(base string, page int) string {
	if page == 1 {
		return base
	}

	return fmt.Sprintf("%s/page/%d", base, page)
}

func firstOf(values []string) string {
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

/*
text reads a front matter value as text.
*/
func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	return ""
}

/*
texts reads a front matter list of text.
*/
func texts(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))

	for _, item := range list {
		out = append(out, text(item))
	}

	return out
}

/*
counts reads a front matter map of whole numbers, such as reaction
counts.
*/
func counts(v any) map[string]int {
	m, _ := v.(map[string]any)
	out := make(map[string]int, len(m))

	for k, n := range m {
		switch n := n.(type) {
		case int:
			out[k] = n
		case int64:
			out[k] = int(n)
		case float64:
			out[k] = int(n)
		}
	}

	return out
}

/*
date reads a front matter date written as YYYY-MM-DD, which YAML may
already have turned into a time.
*/
func date(v any) (time.Time, error) {
	if t, ok := v.(time.Time); ok {
		return t, nil
	}

	t, err := time.Parse(time.DateOnly, cmp.Or(text(v), "missing"))
	if err != nil {
		return time.Time{}, fmt.Errorf("read date: %w", err)
	}

	return t, nil
}
