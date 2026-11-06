/*
|--------------------------------------------------------------------------
| Put Upload Content
|--------------------------------------------------------------------------
|
| On a disk that cannot sign upload addresses, the browser sends the bytes
| to the server, which streams them to the disk. The stream stops the
| moment it passes the size the upload announced, so an upload never
| stores more than it was allowed, and a stopped stream leaves nothing.
|
*/

package attachments

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
errOverSize stops a stream that passed the announced size.
*/
var errOverSize = errors.New("upload over its announced size")

/*
UploadRef names one upload of an uploader.
*/
type UploadRef struct {
	WorkspaceID ids.UUID
	Uploader    Uploader
	ID          ids.UUID
}

/*
PutUploadContent stores the bytes of pending uploads.
*/
type PutUploadContent struct {
	DB   *database.DB
	Disk filesystem.Filesystem
}

/*
Handle streams the body to the upload's place on the disk. It answers
[exceptions.ErrAttachmentNotFound] for an upload that is not the
uploader's, [exceptions.ErrInvalidTransition] once the upload is complete
and [exceptions.ErrAttachmentTooLarge] when the body is larger than
announced.
*/
func (a *PutUploadContent) Handle(ctx context.Context, ref UploadRef, body io.Reader) error {
	row, err := findUpload(ctx, a.DB, ref)
	if err != nil {
		return err
	}

	if row.UploadedAt != nil {
		return exceptions.ErrInvalidTransition
	}

	capped := &sizeCap{r: body, left: row.SizeBytes}

	err = a.Disk.PutStream(ctx, row.Path, capped, filesystem.PutOptions{ContentType: row.ContentType})
	if errors.Is(err, errOverSize) {
		return exceptions.ErrAttachmentTooLarge
	}

	if err != nil {
		return fmt.Errorf("put upload %s: %w: %w", row.ID, exceptions.ErrStorageUnavailable, err)
	}

	return nil
}

/*
findUpload returns the uploader's upload that no message took yet.
*/
func findUpload(ctx context.Context, db *database.DB, ref UploadRef) (models.Attachment, error) {
	var row models.Attachment

	err := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).FindPendingUpload(ctx, models.FindPendingUploadParams{
			WorkspaceID: ref.WorkspaceID, ID: ref.ID, UploadedByType: ref.Uploader.Type, UploadedByID: ref.Uploader.ID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrAttachmentNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find upload %s: %w", ref.ID, err)
	}

	return row, nil
}

/*
sizeCap reads at most left bytes and fails once the reader has more.
*/
type sizeCap struct {
	r    io.Reader
	left int64
}

/*
Read passes bytes through while they fit and fails on the first byte
past the cap.
*/
func (c *sizeCap) Read(p []byte) (int, error) {
	if int64(len(p)) > c.left+1 {
		p = p[:c.left+1]
	}

	n, err := c.r.Read(p)
	c.left -= int64(n)

	if c.left < 0 {
		return 0, errOverSize
	}

	return n, err
}
