/*
|--------------------------------------------------------------------------
| App Controller
|--------------------------------------------------------------------------
|
| Every path of the browser app answers with the same page, in the
| browser's theme, and the app routes in the browser from there. The page
| holds nothing about the caller, so a browser may keep it but must ask
| again before each use, which picks up a new release at once.
|
| The sign-in pages read secrets from the address, after the #, so they
| are never stored and never name themselves to another site.
|
*/

package web

import (
	"cmp"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/view"
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app/support/theme"
	"github.com/dimmihq/dimmi/resources/views/spa"
)

/*
Entry is the source path of the browser app's entry module.
*/
const Entry = "resources/js/app.tsx"

/*
AppController serves the page the browser app mounts into.
*/
type AppController struct {
	Vite   *vite.Bundle
	Errors fhttp.Errors
}

/*
Show renders the page, or an error when the app's scripts cannot be
linked, such as in a binary built without the frontend.
*/
func (c *AppController) Show(w fhttp.ResponseWriter, r *fhttp.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	c.render(w, r)
}

/*
ShowAuth renders the page for a sign-in page, which nothing may store
and which sends no referrer.
*/
func (c *AppController) ShowAuth(w fhttp.ResponseWriter, r *fhttp.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	c.render(w, r)
}

func (c *AppController) render(w fhttp.ResponseWriter, r *fhttp.Request) {
	page := spa.Shell(cmp.Or(i18n.Locale(r.Context()), "en"), theme.Of(r), c.Vite.Tags(Entry))
	if err := view.Respond(w, r, fhttp.StatusOK, page); err != nil {
		c.Errors.Render(w, r, err)
	}
}
