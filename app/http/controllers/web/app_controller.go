/*
|--------------------------------------------------------------------------
| App Controller
|--------------------------------------------------------------------------
|
| Every path of the browser app answers with the same page, and the app
| routes in the browser from there. The page holds nothing about the
| caller, so a browser may keep it but must ask again before each use,
| which picks up a new release at once.
|
*/

package web

import (
	"cmp"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/view"
	"github.com/dimmihq/dimmi/framework/view/vite"

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

	err := view.Respond(w, r, fhttp.StatusOK, spa.Shell(cmp.Or(i18n.Locale(r.Context()), "en"), c.Vite.Tags(Entry)))
	if err != nil {
		c.Errors.Render(w, r, err)
	}
}
