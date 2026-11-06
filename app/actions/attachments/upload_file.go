/*
|--------------------------------------------------------------------------
| Upload File
|--------------------------------------------------------------------------
|
| A teammate's browser sends a file in one request. It goes through the
| same steps as a visitor's three: the file is announced and checked
| against the limits, its bytes are stored, then they are inspected and
| the upload is marked finished. A file refused at any step leaves no
| upload behind.
|
*/

package attachments

import (
	"context"
	"errors"
	"io"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
FileUpload is a file sent in one request: whose workspace it joins, who
sent it, its name, type and size, the most it may be, and its bytes.
*/
type FileUpload struct {
	WorkspaceID ids.UUID
	Uploader    Uploader
	Filename    string
	ContentType string
	SizeBytes   int64
	MaxBytes    int64
	Body        io.Reader
}

/*
UploadFile stores a file sent in one request and returns it finished.

Errors: [exceptions.ErrAttachmentTooLarge],
[exceptions.ErrAttachmentTypeBlocked],
[exceptions.ErrAttachmentSizeMismatch], or a wrapped error.
*/
type UploadFile struct {
	Create   *CreatePendingUpload
	Put      *PutUploadContent
	Complete *CompleteUpload
}

/*
Handle runs the upload's steps in order.
*/
func (a *UploadFile) Handle(ctx context.Context, in FileUpload) (models.Attachment, error) {
	pending, err := a.Create.Handle(ctx, PendingUploadInput{
		WorkspaceID: in.WorkspaceID, Uploader: in.Uploader, Filename: in.Filename,
		ContentType: in.ContentType, SizeBytes: in.SizeBytes, MaxBytes: in.MaxBytes,
	})
	if err != nil {
		return models.Attachment{}, err
	}

	ref := UploadRef{WorkspaceID: in.WorkspaceID, Uploader: in.Uploader, ID: pending.Attachment.ID}
	if err := a.Put.Handle(ctx, ref, in.Body); err != nil {
		return models.Attachment{}, errors.Join(err, a.discard(ctx, pending.Attachment))
	}

	return a.Complete.Handle(ctx, ref)
}

/*
discard drops an upload whose bytes never arrived.
*/
func (a *UploadFile) discard(ctx context.Context, row models.Attachment) error {
	return a.Complete.discard(ctx, row, nil)
}
