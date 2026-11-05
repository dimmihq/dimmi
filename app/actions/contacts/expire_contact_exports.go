/*
|--------------------------------------------------------------------------
| Expire Contact Exports
|--------------------------------------------------------------------------
|
| A customer's export holds everything about them, so it is kept only as
| long as someone needs to download it: a day after it is ready the zip
| is deleted and the export is marked expired. The round finds expired
| exports across workspaces through a function the database lets read
| them all, then works on each inside its own workspace. A zip gone
| already does not stop the round.
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
	"github.com/dimmihq/dimmi/framework/tenancy"

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
Expired exports are found across workspaces, then each is expired
inside its own workspace.
*/
func (a *ExpireContactExports) Handle(ctx context.Context) (int, error) {
	rows, err := models.New(a.DB).ListExpiredContactExports(ctx, models.ListExpiredContactExportsParams{
		Now: a.Clock.Now(), RowLimit: expireBatch,
	})
	if err != nil {
		return 0, fmt.Errorf("list expired exports: %w", err)
	}

	for _, row := range rows {
		scoped := tenancy.With(ctx, row.WorkspaceID.String())

		if err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			return a.expire(ctx, models.New(tx), row)
		}); err != nil {
			return 0, err
		}
	}

	return len(rows), nil
}

/*
expire deletes the export's zip, one gone already included, and marks
the export expired.
*/
func (a *ExpireContactExports) expire(
	ctx context.Context, q *models.Queries, ref models.ListExpiredContactExportsRow,
) error {
	row, err := q.FindContactExportByID(ctx, models.FindContactExportByIDParams(ref))
	if err != nil {
		return fmt.Errorf("find export: %w", err)
	}

	if err := a.deleteZip(ctx, row); err != nil {
		return err
	}

	if err := q.ExpireContactExport(ctx, models.ExpireContactExportParams{
		Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	}); err != nil {
		return fmt.Errorf("expire export: %w", err)
	}

	return nil
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
