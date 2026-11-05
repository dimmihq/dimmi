/*
|--------------------------------------------------------------------------
| Expire Contact Exports
|--------------------------------------------------------------------------
|
| A customer's export holds everything about them, so it is kept only as
| long as someone needs to download it: a day after it is ready the zip
| is deleted and the export is marked expired. A zip gone already does
| not stop the round.
|
*/

package contacts

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/models"
)

/*
expireBatch is the most exports one round expires.
*/
const expireBatch = 500

/*
ExpireContactExports deletes the zips of exports past their expiry.
*/
type ExpireContactExports struct {
	DB      *database.DB
	Clock   clock.Clock
	Storage *filesystem.Manager
}

/*
Handle expires a batch of exports and returns how many it expired.
*/
func (a *ExpireContactExports) Handle(ctx context.Context) (int, error) {
	q := models.New(a.DB)

	rows, err := q.ListExpiredContactExports(ctx, models.ListExpiredContactExportsParams{
		Now: new(a.Clock.Now()), RowLimit: expireBatch,
	})
	if err != nil {
		return 0, fmt.Errorf("list expired exports: %w", err)
	}

	for _, row := range rows {
		if err := a.deleteZip(ctx, row); err != nil {
			return 0, err
		}

		if err := q.ExpireContactExport(ctx, models.ExpireContactExportParams{
			Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		}); err != nil {
			return 0, fmt.Errorf("expire export: %w", err)
		}
	}

	return len(rows), nil
}

/*
deleteZip removes the export's zip, one gone already included.
*/
func (a *ExpireContactExports) deleteZip(ctx context.Context, row models.ContactExport) error {
	if row.Disk == nil || row.Path == nil {
		return nil
	}

	disk, err := a.Storage.Disk(*row.Disk)
	if err != nil {
		return fmt.Errorf("open disk %s: %w", *row.Disk, err)
	}

	if err := disk.Delete(ctx, *row.Path); err != nil && !errors.Is(err, filesystem.ErrNotFound) {
		return fmt.Errorf("delete export: %w", err)
	}

	return nil
}
