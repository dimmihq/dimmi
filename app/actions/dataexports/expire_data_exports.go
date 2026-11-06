/*
|--------------------------------------------------------------------------
| Expire Data Exports
|--------------------------------------------------------------------------
|
| A file leaves the workspace's storage once its export period runs out.
| The export stays listed as expired, so its member knows what happened
| to it.
|
*/

package dataexports

import (
	"context"
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
ExpireDataExports deletes the files of exports past their expiry.
*/
type ExpireDataExports struct {
	DB      *database.DB
	Clock   clock.Clock
	Storage *filesystem.Manager
}

/*
Handle expires a batch of exports and returns how many it expired.
Expired exports are found across workspaces, then each is expired
inside its own workspace.
*/
func (a *ExpireDataExports) Handle(ctx context.Context) (int, error) {
	rows, err := models.New(a.DB).ListExpiredDataExports(ctx, models.ListExpiredDataExportsParams{
		Now: a.Clock.Now(), RowLimit: expireBatch,
	})
	if err != nil {
		return 0, fmt.Errorf("list expired exports: %w", err)
	}

	for _, ref := range rows {
		scoped := tenancy.With(ctx, ref.WorkspaceID.String())

		if err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			return a.expire(ctx, models.New(tx), ref)
		}); err != nil {
			return 0, err
		}
	}

	return len(rows), nil
}

/*
expire deletes the export's file and marks the export expired.
*/
func (a *ExpireDataExports) expire(ctx context.Context, q *models.Queries, ref models.ListExpiredDataExportsRow) error {
	row, err := q.FindDataExport(ctx, models.FindDataExportParams(ref))
	if err != nil {
		return fmt.Errorf("find export: %w", err)
	}

	if err := removeFile(ctx, a.Storage, row); err != nil {
		return err
	}

	if err := q.ExpireDataExport(ctx, models.ExpireDataExportParams{
		Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	}); err != nil {
		return fmt.Errorf("expire export: %w", err)
	}

	return nil
}
