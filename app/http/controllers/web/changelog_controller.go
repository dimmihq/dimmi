/*
|--------------------------------------------------------------------------
| Changelog Controller
|--------------------------------------------------------------------------
|
| The public changelog under its help center's /changelog: the list of
| entries a page at a time, the same filtered to a label, and each entry
| on its own page. Entries come from the sample help center for now. The
| subscribe and reaction forms answer as the real ones will, returning
| the visitor to the page they were on, without storing anything yet.
|
*/

package web

import (
	"strconv"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/resources/views/changelog"
)

/*
ChangelogController serves the public changelog's pages. It shares the
help center's library, styles and not found page.
*/
type ChangelogController struct {
	*HelpCenterController
}

/*
Index shows a page of the newest entries, filtered to a label when the
path names one.
*/
func (c ChangelogController) Index(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	page := 1
	if n := r.PathValue("page"); n != "" {
		parsed, err := strconv.Atoi(n)
		if err != nil || parsed < 2 {
			c.NotFound(w, r)

			return
		}

		page = parsed
	}

	index, found, err := c.Library.Changelog(center, r.PathValue("label"), page)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	if !found {
		c.NotFound(w, r)

		return
	}

	index.Subscribed = r.URL.Query().Has("subscribed")
	c.respond(w, r, fhttp.StatusOK, changelog.IndexPage(center, index, c.styles()))
}

/*
Entry shows one entry, or its Markdown when the path ends in .md.
*/
func (c ChangelogController) Entry(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	slug, markdown := strings.CutSuffix(r.PathValue("slug"), ".md")

	page, found, err := c.Library.Entry(center, slug)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	if !found {
		c.NotFound(w, r)

		return
	}

	if markdown {
		source, _ := c.Library.EntryMarkdown(slug)
		writeMarkdown(w, "# "+page.Entry.Title+"\n\n> "+page.Entry.Summary+"\n\n"+source)

		return
	}

	c.respond(w, r, fhttp.StatusOK, changelog.EntryPage(center, c.Library.Labels(), page, c.styles()))
}

/*
Subscribe takes an email address for announcements and shows the
changelog asking the visitor to confirm from their inbox.
*/
func (c ChangelogController) Subscribe(w fhttp.ResponseWriter, r *fhttp.Request) {
	back(w, strings.TrimSuffix(r.URL.Path, "/subscriptions")+"?subscribed#main")
}

/*
React takes a reaction to an entry and returns to the entry.
*/
func (c ChangelogController) React(w fhttp.ResponseWriter, r *fhttp.Request) {
	back(w, strings.TrimSuffix(r.URL.Path, "/reactions"))
}
