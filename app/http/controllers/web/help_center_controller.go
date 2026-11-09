/*
|--------------------------------------------------------------------------
| Help Center Controller
|--------------------------------------------------------------------------
|
| The public help center under the app's /hc/{center} path, or on its
| own subdomain of the help domain: its home, collections, articles,
| search and the Markdown twin of every article. Pages read the
| workspace's published articles; the sample help center answers only
| where no real one exists, as a demo. A help center or page that does
| not exist answers with the help center's own not found page, never
| with anything that reveals what else exists.
|
| The plain forms on articles post here too. With nothing stored yet,
| they send the visitor back to the page they came from in its answered
| state, so the pages can be tried end to end without scripts.
|
*/

package web

import (
	"errors"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/view"
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/support/helpsample"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
PortalEntry and PortalStyles are the source paths of the public portal's
script and stylesheet. The stylesheet is linked on its own, ahead of the
script, so pages paint styled.
*/
const (
	PortalEntry  = "resources/js/portal/portal.ts"
	PortalStyles = "resources/js/portal/portal.css"
)

/*
HelpCenterController serves the public help center's pages. Assets
serves the images they show; Readers signs readers in through the
team's product. Changelog reads the help center's published changelog
and Subscribers takes the people who want to hear about it. Board is
the board of feature requests. AppURL is where the messenger's loader
and API answer, which a help center on its own host loads across.
*/
type HelpCenterController struct {
	Library     *helpsample.Library
	Public      *helpcenters.Public
	Readers     *helpcenters.Readers
	Changelog   *changelog.Public
	Subscribers *changelog.Subscribers
	Board       *feedback.Posts
	Assets      fhttp.Handler
	Vite        *vite.Bundle
	Errors      fhttp.Errors
	AppURL      string
}

/*
withMessenger has the page load the help center's messenger, if it
shows one, from the app's own address.
*/
func (c *HelpCenterController) withMessenger(center *helpcenter.Center, snap helpcenters.Snapshot) {
	if snap.Messenger == "" {
		return
	}
	center.Messenger = snap.Messenger
	center.MessengerSrc = strings.TrimSuffix(c.AppURL, "/") + "/widget/v1/loader.js"
}

/*
Home shows the help center's front page.
*/
func (c *HelpCenterController) Home(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, lib, ok := c.site(w, r)
	if !ok {
		return
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.Home(
		center, lib.Collections(), lib.Popular(), lib.Updates(3), c.styles(),
	))
}

/*
Collection shows a collection and its articles.
*/
func (c *HelpCenterController) Collection(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, lib, ok := c.site(w, r)
	if !ok {
		return
	}

	col, trail, found := lib.Collection(r.PathValue("slug"))
	if !found {
		c.missing(w, r, center, lib)

		return
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.CollectionPage(center, lib.Collections(), col, trail, c.styles()))
}

/*
Article shows an article, or its Markdown when the path ends in .md.
*/
func (c *HelpCenterController) Article(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, lib, ok := c.site(w, r)
	if !ok {
		return
	}

	slug, markdown := strings.CutSuffix(r.PathValue("slug"), ".md")

	article, found, err := lib.Article(center, slug)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	if !found {
		c.missing(w, r, center, lib)

		return
	}

	if markdown {
		source, _ := lib.Markdown(slug)
		writeMarkdown(w, "# "+article.Title+"\n\n> "+article.Description+"\n\n"+source)

		return
	}

	page := helpcenter.ArticlePage(
		center, lib.Collections(), article, r.URL.Query().Get("helpful"), c.styles(),
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
	center, lib, ok := c.site(w, r)
	if !ok {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page := helpcenter.SearchPage(center, lib.Collections(), query, lib.Search(query), c.styles())
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
	center, lib, ok := c.site(w, r)
	if !ok {
		return
	}

	c.missing(w, r, center, lib)
}

/*
missing answers with the help center's not found page.
*/
func (c *HelpCenterController) missing(w fhttp.ResponseWriter, r *fhttp.Request, center helpcenter.Center,
	lib HelpLibrary,
) {
	if s, ok := lib.(*site); ok {
		rest := strings.TrimPrefix(r.URL.Path, "/hc/"+r.PathValue("center"))
		if to, found := c.Public.Redirect(r.Context(), s.snap.HelpCenter, rest); found {
			if strings.HasPrefix(to, "/") {
				to = center.Base + to
			}
			w.Header().Set("Location", to)
			w.WriteHeader(fhttp.StatusMovedPermanently)
			return
		}
	}
	page := helpcenter.NotFoundPage(center, lib.Collections(), lib.Popular(), c.styles())
	c.respond(w, r, fhttp.StatusNotFound, page)
}

/*
site finds the help center the request names and what its pages read:
a real help center when one answers the name, else the sample's demo.
*/
func (c *HelpCenterController) site(w fhttp.ResponseWriter, r *fhttp.Request,
) (helpcenter.Center, HelpLibrary, bool) {
	slug, base := r.PathValue("center"), "/hc/"+r.PathValue("center")
	if hostedHere(r.Context()) {
		base = ""
	}
	if c.Public != nil {
		snap, err := c.Public.Load(r.Context(), slug)
		if err == nil {
			return c.realSite(w, r, snap, base)
		}
		if !errors.Is(err, helpcenters.ErrNoHelpCenter) {
			c.Errors.Render(w, r, err)
			return helpcenter.Center{}, nil, false
		}
	}
	center, ok := c.center(w, r)
	return center, c.Library, ok
}

func origin(r *fhttp.Request) string {
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
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
	return c.Vite.Tags(PortalStyles, PortalEntry)
}

func (c *HelpCenterController) respond(w fhttp.ResponseWriter, r *fhttp.Request, status int, page view.Component) {
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-cache")
	}

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
