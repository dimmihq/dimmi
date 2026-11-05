/*
|--------------------------------------------------------------------------
| Create Pending Upload
|--------------------------------------------------------------------------
|
| A browser sends a file straight to storage rather than through the
| server, so an upload starts as a row with no bytes. The row is refused
| up front when the announced file is too large or of a blocked kind; once
| it exists, the browser gets a short-lived signed address to put the
| bytes at, or, on a disk that cannot sign one, is told to send them to
| the server itself.
|
*/

package attachments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
uploadLinkTTL is how long a signed upload address accepts the bytes.
*/
const uploadLinkTTL = 15 * time.Minute

/*
Uploader names who makes an upload: a kind, such as a widget session, and
its id. Only the same uploader may send the bytes, complete the upload or
attach it.
*/
type Uploader struct {
	Type string
	ID   ids.UUID
}

/*
PendingUploadInput describes the file about to be uploaded. MaxBytes is
the largest file the place it is sent from accepts.
*/
type PendingUploadInput struct {
	WorkspaceID ids.UUID
	Uploader    Uploader
	Filename    string
	ContentType string
	SizeBytes   int64
	MaxBytes    int64
}

/*
PendingUpload is an upload waiting for its bytes. URL is the signed
address that takes one PUT of them; when Direct is true there is none and
the bytes go to the server's own upload route.
*/
type PendingUpload struct {
	Attachment models.Attachment
	URL        string
	Direct     bool
	ExpiresAt  time.Time
}

/*
CreatePendingUpload starts uploads on a disk.
*/
type CreatePendingUpload struct {
	DB       *database.DB
	Clock    clock.Clock
	Disk     filesystem.Filesystem
	DiskName string
}

/*
Handle records the upload and returns where its bytes go, or
[exceptions.ErrAttachmentTooLarge] or [exceptions.ErrAttachmentTypeBlocked]
when the announced file is refused.
*/
func (a *CreatePendingUpload) Handle(ctx context.Context, in PendingUploadInput) (PendingUpload, error) {
	if in.SizeBytes > in.MaxBytes {
		return PendingUpload{}, exceptions.ErrAttachmentTooLarge
	}

	if blockedExtension(in.Filename) || blockedType(in.ContentType) {
		return PendingUpload{}, exceptions.ErrAttachmentTypeBlocked
	}

	now := a.Clock.Now()
	path := jobs.WorkspaceStoragePrefix(in.WorkspaceID) + "attachments/" + ids.NewV7At(now).String()

	var row models.Attachment

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).CreateAttachment(ctx, models.CreateAttachmentParams{
			WorkspaceID: in.WorkspaceID, UploadedByType: in.Uploader.Type, UploadedByID: &in.Uploader.ID,
			Disk: a.DiskName, Path: path, Filename: in.Filename, ContentType: in.ContentType,
			SizeBytes: in.SizeBytes, ScanStatus: "pending", Now: now,
		})

		return err
	})
	if err != nil {
		return PendingUpload{}, fmt.Errorf("create pending upload: %w", err)
	}

	return a.instructions(ctx, row, now)
}

/*
instructions signs the address the bytes go to, or answers a direct upload
on a disk that signs nothing.
*/
func (a *CreatePendingUpload) instructions(
	ctx context.Context, row models.Attachment, now time.Time,
) (PendingUpload, error) {
	pending := PendingUpload{Attachment: row, ExpiresAt: now.Add(uploadLinkTTL)}

	url, err := a.Disk.TemporaryUploadURL(ctx, row.Path, filesystem.UploadURLOptions{
		TTL: uploadLinkTTL, ContentType: row.ContentType,
	})
	if errors.Is(err, filesystem.ErrUnsupported) {
		pending.Direct = true

		return pending, nil
	}

	if err != nil {
		return PendingUpload{}, fmt.Errorf("sign upload %s: %w", row.ID, err)
	}

	pending.URL = url

	return pending, nil
}
