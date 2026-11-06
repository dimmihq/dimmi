/*
|--------------------------------------------------------------------------
| Attachment Controller
|--------------------------------------------------------------------------
|
| POST /attachments takes a file a teammate is about to send, in one
| request: the body is the file, the Content-Type header its type and the
| filename query parameter its name. The body streams straight to the
| disk rather than through the API layer, which holds whole bodies in
| memory and caps them far below a file's size. The answer is the
| finished upload, which a reply then names.
|
*/

package v1

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/attachments"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
MaxMemberUploadBytes is the largest file a teammate may attach, the most
mail providers accept in one message.
*/
const MaxMemberUploadBytes = 25 << 20

/*
AttachmentController takes teammates' uploads.
*/
type AttachmentController struct {
	Upload *attachments.UploadFile
	Disks  messages.Disks
	Errors fhttp.Errors
}

/*
Store saves the file and answers 201 with the finished upload.
*/
func (c *AttachmentController) Store(w fhttp.ResponseWriter, r *fhttp.Request) {
	ctx := r.Context()
	m := membership(ctx)

	if err := conversations.CanWrite(m); err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" || r.ContentLength <= 0 {
		c.Errors.Render(w, r, exceptions.ErrAttachmentSizeMismatch)

		return
	}

	row, err := c.Upload.Handle(ctx, attachments.FileUpload{
		WorkspaceID: m.Workspace.ID,
		Uploader:    attachments.Uploader{Type: messages.UploaderMember, ID: m.Member.ID},
		Filename:    filename, ContentType: contentType(r), SizeBytes: r.ContentLength,
		MaxBytes: MaxMemberUploadBytes, Body: fhttp.MaxBytesReader(w, r.Body, MaxMemberUploadBytes+1),
	})
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	file := resources.NewAttachment(messages.PresentFile(ctx, c.Disks, row))
	fhttp.WriteJSON(w, fhttp.StatusCreated, fhttp.NewResource(file))
}

/*
contentType is the type the browser gave the file, or a plain byte
stream when it gave none.
*/
func contentType(r *fhttp.Request) string {
	if t := strings.TrimSpace(r.Header.Get("Content-Type")); t != "" {
		return t
	}

	return "application/octet-stream"
}
