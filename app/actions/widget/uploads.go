/*
|--------------------------------------------------------------------------
| Visitor Uploads
|--------------------------------------------------------------------------
|
| A visitor attaches files in three steps: announce the file, send its
| bytes, then say they arrived. Every step belongs to the session that
| started it, so no visitor can touch another's upload, and each step
| obeys the widget's own switch and size limit, read fresh on every
| request so a change in the dashboard takes effect at once.
|
*/

package widget

import (
	"context"
	"io"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/attachments"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UploaderWidgetSession is the uploader kind of files visitors send.
*/
const UploaderWidgetSession = "widget_session"

/*
UploadFile is the file a visitor announces.
*/
type UploadFile struct {
	Filename    string
	ContentType string
	SizeBytes   int64
}

/*
Uploads runs visitors' uploads on the widget's disk.
*/
type Uploads struct {
	Create   *attachments.CreatePendingUpload
	Put      *attachments.PutUploadContent
	Complete *attachments.CompleteUpload
	Disk     filesystem.Filesystem
	Limits   *VisitorLimits
}

/*
Start records the announced file and returns where its bytes go, or
[exceptions.ErrAttachmentsDisabled] when the widget takes no files.
*/
func (a *Uploads) Start(
	ctx context.Context, auth Authenticated, file UploadFile,
) (attachments.PendingUpload, error) {
	if !auth.Resolved.Widget.AttachmentsEnabled {
		return attachments.PendingUpload{}, exceptions.ErrAttachmentsDisabled
	}

	if err := a.Limits.Upload(ctx, auth); err != nil {
		return attachments.PendingUpload{}, err
	}

	return a.Create.Handle(ctx, attachments.PendingUploadInput{
		WorkspaceID: auth.Session.WorkspaceID, Uploader: uploader(auth.Session),
		Filename: file.Filename, ContentType: file.ContentType, SizeBytes: file.SizeBytes,
		MaxBytes: MaxUploadBytes(auth.Resolved.Widget),
	})
}

/*
Send stores the bytes of the session's upload.
*/
func (a *Uploads) Send(ctx context.Context, auth Authenticated, id ids.UUID, body io.Reader) error {
	if !auth.Resolved.Widget.AttachmentsEnabled {
		return exceptions.ErrAttachmentsDisabled
	}

	return a.Put.Handle(ctx, uploadRef(auth.Session, id), body)
}

/*
Finish checks the bytes of the session's upload and marks it uploaded.
*/
func (a *Uploads) Finish(ctx context.Context, auth Authenticated, id ids.UUID) (VisibleAttachment, error) {
	if !auth.Resolved.Widget.AttachmentsEnabled {
		return VisibleAttachment{}, exceptions.ErrAttachmentsDisabled
	}

	row, err := a.Complete.Handle(ctx, uploadRef(auth.Session, id))
	if err != nil {
		return VisibleAttachment{}, err
	}

	return presentAttachment(ctx, a.Disk, row), nil
}

/*
MaxUploadBytes is the largest file the widget accepts.
*/
func MaxUploadBytes(w models.WidgetSetting) int64 {
	return int64(w.MaxAttachmentMb) << 20
}

/*
uploader names the session as the uploader of its files.
*/
func uploader(s models.WidgetSession) attachments.Uploader {
	return attachments.Uploader{Type: UploaderWidgetSession, ID: s.ID}
}

/*
uploadRef names one of the session's uploads.
*/
func uploadRef(s models.WidgetSession, id ids.UUID) attachments.UploadRef {
	return attachments.UploadRef{WorkspaceID: s.WorkspaceID, Uploader: uploader(s), ID: id}
}
