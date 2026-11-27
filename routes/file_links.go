/*
|--------------------------------------------------------------------------
| File Link Routes
|--------------------------------------------------------------------------
|
| The download link a reply email carries for a file too large to
| attach. It works signed out: the application signed it, and the
| signature covers the workspace and the file, so nothing in it can be
| changed.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
)

/*
FileDownloadRoute names the route of an emailed file's download link.
*/
const FileDownloadRoute = "files.download"

/*
fileLinks declares the download links of emailed files.
*/
func fileLinks(a *app.App) {
	files := web.FileDownloadController{
		Download: &messages.DownloadFile{DB: a.DB, Disks: a.Storage}, IDs: a.IDs, Errors: a.Errors,
	}

	a.Router.Handle(fhttp.MethodGet, "/files/{workspace}/{attachment}", FileDownloadRoute,
		fhttp.HandlerFunc(files.Show)).Middleware("signed")
}
