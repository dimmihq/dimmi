/*
|--------------------------------------------------------------------------
| File Downloads
|--------------------------------------------------------------------------
|
| The signed link an email carries for a file too large to attach. The
| signature is checked before this runs; it sends the browser on to a
| short-lived link to the stored file, and answers not found for a file
| that is gone or that the scan refused.
|
*/

package web

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
FileDownloadController answers the download links of emailed files.
*/
type FileDownloadController struct {
	Download *messages.DownloadFile
	IDs      *ids.Registry
	Errors   fhttp.Errors
}

/*
Show redirects to the file.
*/
func (c FileDownloadController) Show(w fhttp.ResponseWriter, r *fhttp.Request) {
	ws, err := c.IDs.Decode(string(models.PrefixWorkspace), r.PathValue("workspace"))
	if err != nil {
		c.Errors.Render(w, r, exceptions.ErrNotFound)

		return
	}

	file, err := c.IDs.Decode(string(models.PrefixAttachment), r.PathValue("attachment"))
	if err != nil {
		c.Errors.Render(w, r, exceptions.ErrNotFound)

		return
	}

	url, err := c.Download.Handle(r.Context(), ws, file)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Location", url)
	w.WriteHeader(fhttp.StatusFound)
}
