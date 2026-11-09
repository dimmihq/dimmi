/*
|--------------------------------------------------------------------------
| Changelog Controller
|--------------------------------------------------------------------------
|
| The public changelog under its help center's /changelog: the list of
| entries a page at a time, the same filtered to a label, each entry on
| its own page, and the RSS feed. A real help center shows its published
| entries; the sample help center answers only where no real one
| exists, as a demo. Subscribing and reacting are kept for real help
| centers, and the sample answers them without storing anything.
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
	page := 1
	if n := r.PathValue("page"); n != "" {
		parsed, err := strconv.Atoi(n)
		if err != nil || parsed < 2 {
			c.NotFound(w, r)

			return
		}

		page = parsed
	}

	if site, live, ok := c.live(w, r); live || !ok {
		if ok {
			c.realIndex(w, r, site, page)
		}

		return
	}

	center, ok := c.center(w, r)
	if !ok {
		return
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
	slug, markdown := strings.CutSuffix(r.PathValue("slug"), ".md")

	if site, live, ok := c.live(w, r); live || !ok {
		if ok {
			c.realEntry(w, r, site)
		}

		return
	}

	center, ok := c.center(w, r)
	if !ok {
		return
	}

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
