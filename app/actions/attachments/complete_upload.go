/*
|--------------------------------------------------------------------------
| Complete Upload
|--------------------------------------------------------------------------
|
| Once the browser says the bytes are in place, the server checks them
| itself before any message may carry the file: it must be there, exactly
| the size announced, not a program or a page whatever its name says, and
| a picture small enough to show. A file that fails is removed with its
| row, so the browser starts over. Pictures have their size read from the
| header only; nothing is ever fully decoded here.
|
| Completing twice is harmless: the second time answers the upload as it
| stands, so a client may retry after losing the answer.
|
*/

package attachments

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/image"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
sniffLen is how many leading bytes decide what a file really is.
*/
const sniffLen = 512

/*
CompleteUpload checks and finishes uploads.
*/
type CompleteUpload struct {
	DB    *database.DB
	Clock clock.Clock
	Disk  filesystem.Filesystem
	Queue *queue.Queue
}

/*
inspected is what reading an upload's bytes found.
*/
type inspected struct {
	contentType string
	width       *int32
	height      *int32
	sum         string
}

/*
Handle checks the upload's bytes and marks it uploaded. Missing bytes
answer [exceptions.ErrAttachmentNotUploaded]; a file of another size,
of a blocked kind or with too many pixels is removed and answers
[exceptions.ErrAttachmentSizeMismatch],
[exceptions.ErrAttachmentTypeBlocked] or
[exceptions.ErrAttachmentImageTooLarge].
*/
func (a *CompleteUpload) Handle(ctx context.Context, ref UploadRef) (models.Attachment, error) {
	row, err := findUpload(ctx, a.DB, ref)
	if err != nil || row.UploadedAt != nil {
		return row, err
	}

	attrs, err := a.Disk.Stat(ctx, row.Path)
	if errors.Is(err, filesystem.ErrNotFound) {
		return row, exceptions.ErrAttachmentNotUploaded
	}

	if err != nil {
		return row, fmt.Errorf("stat upload %s: %w: %w", row.ID, exceptions.ErrStorageUnavailable, err)
	}

	if attrs.Size != row.SizeBytes {
		return row, a.discard(ctx, row, exceptions.ErrAttachmentSizeMismatch)
	}

	found, err := a.inspect(ctx, row)
	if err != nil {
		return row, err
	}

	return a.finish(ctx, row, found)
}

/*
inspect reads the upload once: its first bytes tell what it is, a
picture's header gives its size, and every byte feeds the checksum.
*/
func (a *CompleteUpload) inspect(ctx context.Context, row models.Attachment) (inspected, error) {
	file, err := a.Disk.Open(ctx, row.Path)
	if err != nil {
		return inspected{}, fmt.Errorf("open upload %s: %w: %w", row.ID, exceptions.ErrStorageUnavailable, err)
	}
	defer func() { _ = file.Close() }()

	digest := hashing.NewSHA256()
	stream := io.TeeReader(file, digest)

	head := make([]byte, sniffLen)

	n, err := io.ReadFull(stream, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return inspected{}, fmt.Errorf("read upload %s: %w: %w", row.ID, exceptions.ErrStorageUnavailable, err)
	}

	head = head[:n]

	if executableHeader(head) || blockedType(filesystem.DetectContentType(head)) {
		return inspected{}, a.discard(ctx, row, exceptions.ErrAttachmentTypeBlocked)
	}

	found := inspected{contentType: row.ContentType}

	if format, ok := pictureFormat(head); ok {
		found.contentType = format.ContentType()

		if err := a.measure(ctx, row, io.MultiReader(bytes.NewReader(head), stream), &found); err != nil {
			return inspected{}, err
		}
	}

	if _, err := io.Copy(io.Discard, stream); err != nil {
		return inspected{}, fmt.Errorf("read upload %s: %w: %w", row.ID, exceptions.ErrStorageUnavailable, err)
	}

	found.sum = hex.EncodeToString(digest.Sum(nil))

	return found, nil
}

/*
pictureFormat names the picture format the first bytes start, when they
start one.
*/
func pictureFormat(head []byte) (image.Format, bool) {
	format, err := image.Sniff(bytes.NewReader(head))

	return format, err == nil
}

/*
measure reads a picture's width and height from its header. A picture
with too many pixels, or bytes that only pretend to be one, is removed.
*/
func (a *CompleteUpload) measure(
	ctx context.Context, row models.Attachment, picture io.Reader, found *inspected,
) error {
	cfg, err := image.DecodeConfig(picture, image.Limits{MaxBytes: row.SizeBytes + 1})
	if errors.Is(err, image.ErrTooManyPixels) {
		return a.discard(ctx, row, exceptions.ErrAttachmentImageTooLarge)
	}

	if err != nil {
		return a.discard(ctx, row, exceptions.ErrAttachmentTypeBlocked)
	}

	found.width, found.height = side(cfg.Width), side(cfg.Height)

	return nil
}

/*
side stores a picture's width or height, which the pixel limit keeps far
below what the column holds.
*/
func side(n int) *int32 {
	if n < 0 || n > math.MaxInt32 {
		return nil
	}

	v := int32(n)

	return &v
}

/*
finish marks the upload uploaded with what inspecting it found, and
queues its malware scan. Until the scan judges it the upload is pending.
*/
func (a *CompleteUpload) finish(
	ctx context.Context, row models.Attachment, found inspected,
) (models.Attachment, error) {
	now := a.Clock.Now()

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		done, err := models.New(tx).CompleteUpload(ctx, models.CompleteUploadParams{
			Now: &now, ContentType: found.contentType, Width: found.width, Height: found.height, Sha256: &found.sum,
			ScanStatus: "pending", WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			done, err = models.New(tx).FindAttachment(ctx, models.FindAttachmentParams{
				WorkspaceID: row.WorkspaceID, ID: row.ID,
			})

			row = done

			return err
		}

		row = done
		if err != nil {
			return err
		}

		return a.Queue.PushTx(ctx, tx, jobs.ScanAttachmentArgs{WorkspaceID: row.WorkspaceID, AttachmentID: row.ID})
	})
	if err != nil {
		return row, fmt.Errorf("complete upload %s: %w", row.ID, err)
	}

	return row, nil
}

/*
discard removes a refused upload's bytes and row and returns why it was
refused.
*/
func (a *CompleteUpload) discard(ctx context.Context, row models.Attachment, reason error) error {
	if err := a.Disk.Delete(ctx, row.Path); err != nil {
		return fmt.Errorf("delete refused upload %s: %w: %w", row.ID, exceptions.ErrStorageUnavailable, err)
	}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).DeleteUnattachedAttachment(ctx, models.DeleteUnattachedAttachmentParams{
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("delete refused upload %s: %w", row.ID, err)
	}

	return reason
}
