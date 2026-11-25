/*
|--------------------------------------------------------------------------
| Deleting Stored Email
|--------------------------------------------------------------------------
|
| An email addressed to two workspaces is stored once and recorded by
| both, so its file goes only when the last record that points at it
| does. Retention, erasures and workspace purges all delete stored email
| through here.
|
*/

package retention

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
RawEmail is a stored email and the record that points at it.
*/
type RawEmail struct {
	ID   ids.UUID
	Disk string
	Path string
}

/*
DeleteRawEmails deletes the stored emails no record but the given ones
still points at. A file gone already is no error.
*/
func DeleteRawEmails(ctx context.Context, db *database.DB, storage Disks, emails []RawEmail) error {
	if len(emails) == 0 {
		return nil
	}

	going := make([]ids.UUID, 0, len(emails))
	for _, email := range emails {
		going = append(going, email.ID)
	}

	for _, email := range emails {
		var shared bool

		err := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error

			shared, err = models.New(tx).RawEmailShared(ctx, models.RawEmailSharedParams{
				RawDisk: email.Disk, RawPath: email.Path, ExceptIds: going,
			})

			return err
		})
		if err != nil {
			return fmt.Errorf("check raw email: %w", err)
		}

		if shared {
			continue
		}

		disk, err := storage.Disk(email.Disk)
		if err != nil {
			return fmt.Errorf("open disk %s: %w", email.Disk, err)
		}

		if err := disk.Delete(ctx, email.Path); err != nil && !errors.Is(err, filesystem.ErrNotFound) {
			return fmt.Errorf("delete raw email: %w", err)
		}
	}

	return nil
}
