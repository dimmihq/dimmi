/*
|--------------------------------------------------------------------------
| Help Center Controller
|--------------------------------------------------------------------------
|
| The public help center under the app's /hc/{center} path: its home,
| collections, articles, search and the Markdown twin of every article.
| Pages read the sample help center for now, and a help center or page
| that does not exist answers with the help center's own not found page,
| never with anything that reveals what else exists.
|
| The plain forms on articles post here too. With nothing stored yet,
| they send the visitor back to the page they came from in its answered
| state, so the pages can be tried end to end without scripts.
|
*/

package web

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/view"
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app/support/helpsample"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
PortalEntry is the source path of the public portal's script, which
brings its stylesheet.
*/
const PortalEntry = "resources/js/portal/portal.ts"

/*
HelpCenterController serves the public help center's pages. Assets
serves the images they show.
*/
type HelpCenterController struct {
	Library *helpsample.Library
	Assets  fhttp.Handler
	Vite    *vite.Bundle
	Errors  fhttp.Errors
}

/*
Home shows the help center's front page.
*/
func (c *HelpCenterController) Home(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.Home(
		center, c.Library.Collections(), c.Library.Popular(), c.Library.Updates(3), c.styles(),
	))
}

/*
Collection shows a collection and its articles.
*/
func (c *HelpCenterController) Collection(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	col, trail, found := c.Library.Collection(r.PathValue("slug"))
	if !found {
		c.NotFound(w, r)

		return
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.CollectionPage(center, c.Library.Collections(), col, trail, c.styles()))
}

/*
Article shows an article, or its Markdown when the path ends in .md.
*/
func (c *HelpCenterController) Article(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	slug, markdown := strings.CutSuffix(r.PathValue("slug"), ".md")

	article, found, err := c.Library.Article(center, slug)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	if !found {
		c.NotFound(w, r)

		return
	}

	if markdown {
		source, _ := c.Library.Markdown(slug)
		writeMarkdown(w, "# "+article.Title+"\n\n> "+article.Description+"\n\n"+source)

		return
	}

	page := helpcenter.ArticlePage(
		center, c.Library.Collections(), article, r.URL.Query().Get("helpful"), c.styles(),
	)
	c.respond(w, r, fhttp.StatusOK, page)
}

/*
Rate takes the answer to "Was this article helpful?" and returns the
visitor to the article, thanked.
*/
func (c *HelpCenterController) Rate(w fhttp.ResponseWriter, r *fhttp.Request) {
	answer := "yes"
	if r.FormValue("helpful") == "no" {
		answer = "no"
	}

	back(w, strings.TrimSuffix(r.URL.Path, "/feedback")+"?helpful="+answer+"#feedback")
}

/*
Search shows the articles matching the query.
*/
func (c *HelpCenterController) Search(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page := helpcenter.SearchPage(center, c.Library.Collections(), query, c.Library.Search(query), c.styles())
	c.respond(w, r, fhttp.StatusOK, page)
}

/*
Asset serves an image the sample help center's pages show, from the
handler of the sample's files.
*/
func (c *HelpCenterController) Asset(w fhttp.ResponseWriter, r *fhttp.Request) {
	if _, ok := c.center(w, r); !ok {
		return
	}

	file := r.Clone(r.Context())
	file.URL.Path = "/" + r.PathValue("file")
	c.Assets.ServeHTTP(w, file)
}

/*
NotFound answers a path the help center has nothing at.
*/
func (c *HelpCenterController) NotFound(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, ok := c.center(w, r)
	if !ok {
		return
	}

	page := helpcenter.NotFoundPage(center, c.Library.Collections(), c.Library.Popular(), c.styles())
	c.respond(w, r, fhttp.StatusNotFound, page)
}

/*
center finds the portal the path names, served on the request's host,
with the visitor when they signed in. An unknown portal gets a bare not
found, since there is no portal to dress the page in.
*/
func (c *HelpCenterController) center(w fhttp.ResponseWriter, r *fhttp.Request) (helpcenter.Center, bool) {
	slug := r.PathValue("center")

	center, ok := c.Library.Center(slug, "/hc/"+slug)
	if !ok {
		w.WriteHeader(fhttp.StatusNotFound)

		return center, false
	}

	center.Origin = "http://" + r.Host
	if r.TLS != nil {
		center.Origin = "https://" + r.Host
	}

	if cookie, err := r.Cookie(visitorCookie); err == nil && cookie.Value == "1" {
		visitor := helpsample.Visitor()
		center.Visitor = &visitor
	}

	return center, true
}

func (c *HelpCenterController) styles() view.Component {
	return c.Vite.Tags(PortalEntry)
}

func (c *HelpCenterController) respond(w fhttp.ResponseWriter, r *fhttp.Request, status int, page view.Component) {
	w.Header().Set("Cache-Control", "no-cache")

	if err := view.Respond(w, r, status, page); err != nil {
		c.Errors.Render(w, r, err)
	}
}

/*
writeMarkdown sends a page's Markdown twin.
*/
func writeMarkdown(w fhttp.ResponseWriter, source string) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(fhttp.StatusOK)
	_, _ = w.Write([]byte(source))
}

/*
back sends the visitor to a page after a form, so reloading it never
posts the form again.
*/
func back(w fhttp.ResponseWriter, to string) {
	w.Header().Set("Location", to)
	w.WriteHeader(fhttp.StatusSeeOther)
}
