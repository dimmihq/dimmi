/*
|--------------------------------------------------------------------------
| Widget Attachment Controller
|--------------------------------------------------------------------------
|
| A visitor's messenger uploads a file in three requests: announce it,
| send its bytes, and say they arrived. The bytes route streams the body
| straight to the disk rather than through the API layer, which holds
| whole bodies in memory and caps them far below a file's size.
|
*/

package widget

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
statusUploaded is the one state a visitor may move an upload to.
*/
const statusUploaded = "uploaded"

/*
AttachmentUploadOutput is a new pending upload.
*/
type AttachmentUploadOutput struct {
	Status int
	Body   fhttp.Resource[resources.WidgetUpload]
}

/*
AttachmentOutput is a finished upload.
*/
type AttachmentOutput struct {
	Body fhttp.Resource[resources.WidgetAttachment]
}

/*
AttachmentController serves visitors' uploads. BaseURL returns the base
address a request reached this install on; Errors renders the failures
of the bytes route.
*/
type AttachmentController struct {
	Uploads *actions.Uploads
	IDs     *ids.Registry
	Errors  fhttp.Errors
	BaseURL func(ctx context.Context) string
}

/*
Store records the announced file and answers where its bytes go.
*/
func (c *AttachmentController) Store(
	ctx context.Context, in *requests.StoreWidgetAttachmentRequest,
) (*AttachmentUploadOutput, error) {
	pending, err := c.Uploads.Start(ctx, authenticated(ctx), actions.UploadFile{
		Filename: in.Body.Filename, ContentType: in.Body.ContentType, SizeBytes: in.Body.SizeBytes,
	})
	if err != nil {
		return nil, err
	}

	upload := resources.NewWidgetUpload(pending, c.BaseURL(ctx)+"/widget/v1")

	return &AttachmentUploadOutput{Status: fhttp.StatusCreated, Body: fhttp.NewResource(upload)}, nil
}

/*
Update marks the upload uploaded once its bytes pass the checks.
*/
func (c *AttachmentController) Update(
	ctx context.Context, in *requests.UpdateWidgetAttachmentRequest,
) (*AttachmentOutput, error) {
	if in.Body.Status != statusUploaded {
		return nil, exceptions.ErrInvalidTransition.With("to", in.Body.Status)
	}

	id, err := c.attachmentID(in.Attachment)
	if err != nil {
		return nil, err
	}

	file, err := c.Uploads.Finish(ctx, authenticated(ctx), id)
	if err != nil {
		return nil, err
	}

	return &AttachmentOutput{Body: fhttp.NewResource(resources.NewWidgetAttachment(file))}, nil
}

/*
Content streams the request body to the upload's place on the disk and
answers 204, never reading more than the widget's size limit.
*/
func (c *AttachmentController) Content(w fhttp.ResponseWriter, r *fhttp.Request) {
	ctx := r.Context()
	auth := authenticated(ctx)

	id, err := c.attachmentID(r.PathValue("attachment"))
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	body := fhttp.MaxBytesReader(w, r.Body, actions.MaxUploadBytes(auth.Resolved.Widget)+1)

	if err := c.Uploads.Send(ctx, auth, id, body); err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	w.WriteHeader(fhttp.StatusNoContent)
}

/*
attachmentID decodes an upload's public id; anything else is simply not
found.
*/
func (c *AttachmentController) attachmentID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixAttachment), strings.TrimSpace(public))
	if err != nil {
		return id, exceptions.ErrAttachmentNotFound
	}

	return id, nil
}
