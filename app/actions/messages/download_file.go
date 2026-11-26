/*
|--------------------------------------------------------------------------
| Downloading A Reply's File
|--------------------------------------------------------------------------
|
| A file too large to travel with an email is linked from it instead.
| The link is signed by the application and names the workspace and the
| file, so it works without signing in and cannot be bent to another
| file. Following it opens a short-lived link to the stored file, for a
| file a message carries and the malware scan let through.
|
*/

package messages

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
downloadTTL is how long the link to the stored file works once the
signed link was followed.
*/
const downloadTTL = 10 * time.Minute

/*
DownloadFile opens a file of a message for whoever holds its signed
link.

Errors: [exceptions.ErrNotFound] for a file no message carries, one the
scan refused or has not cleared, or a wrapped error.
*/
type DownloadFile struct {
	DB    *database.DB
	Disks Disks
}

/*
Handle returns a short-lived link to the file.
*/
func (a *DownloadFile) Handle(ctx context.Context, workspaceID, attachmentID ids.UUID) (string, error) {
	ctx = tenancy.With(ctx, workspaceID.String())

	var row models.Attachment

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).FindAttachment(ctx, models.FindAttachmentParams{
			WorkspaceID: workspaceID, ID: attachmentID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return "", exceptions.ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf("find file: %w", err)
	}

	if row.MessageID == nil || (row.ScanStatus != "clean" && row.ScanStatus != "skipped") {
		return "", exceptions.ErrNotFound
	}

	disk, err := a.Disks.Disk(row.Disk)
	if err != nil {
		return "", fmt.Errorf("find disk %s: %w", row.Disk, err)
	}

	return Link(ctx, disk, row, downloadTTL)
}
