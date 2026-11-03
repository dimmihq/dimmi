/*
|--------------------------------------------------------------------------
| Widget Asset Controller
|--------------------------------------------------------------------------
|
| The messenger's files load on customers' sites, so every response may
| be read from any origin and carries no cookie. The loader keeps its
| fixed address and is cached only briefly, so a deploy reaches every
| page within minutes; the hashed files it points at are cached forever.
| A binary built without the widget answers 404 for all of them.
|
*/

package widget

import (
	"fmt"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/support/widgetassets"
)

/*
AssetController serves the loader, the manifest and the hashed files.
Assets is nil when the binary has no widget build. LoaderMaxAge is how
long clients keep the loader before asking again.
*/
type AssetController struct {
	Assets       *widgetassets.Assets
	LoaderMaxAge time.Duration
}

/*
Loader serves the script customers paste, filled in with the current core.
*/
func (c *AssetController) Loader(w fhttp.ResponseWriter, r *fhttp.Request) {
	if !c.serving(w) {
		return
	}

	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control",
		fmt.Sprintf("public, max-age=%d, stale-while-revalidate=86400", int(c.LoaderMaxAge.Seconds())))
	c.Assets.Loader.Write(w, r)
}

/*
Manifest serves the build's manifest: its version, files, sizes,
integrity hashes and size budgets.
*/
func (c *AssetController) Manifest(w fhttp.ResponseWriter, r *fhttp.Request) {
	if !c.serving(w) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	c.Assets.Manifest.Write(w, r)
}

/*
Asset serves a hashed file, compressed when the client accepts it.
*/
func (c *AssetController) Asset(w fhttp.ResponseWriter, r *fhttp.Request) {
	if !c.serving(w) {
		return
	}

	c.Assets.Files.ServeHTTP(w, r)
}

/*
serving sets the headers every widget file carries and reports whether
there is a build to serve, answering 404 when there is none.
*/
func (c *AssetController) serving(w fhttp.ResponseWriter) bool {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	h.Set("Timing-Allow-Origin", "*")
	h.Set("X-Content-Type-Options", "nosniff")

	if c.Assets == nil {
		w.WriteHeader(fhttp.StatusNotFound)

		return false
	}

	return true
}
