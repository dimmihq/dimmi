/*
|--------------------------------------------------------------------------
| Expiring Imports
|--------------------------------------------------------------------------
|
| An uploaded file is kept for a month after its import finishes, for
| the report of failed rows, and then emptied; the import and its counts
| stay, and the report says it expired. What an import changed is kept
| only while it can be undone, then dropped too. A nightly sweep finds
| both across workspaces and handles each inside its own workspace.
|
*/

package imports

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
FileKept is how long after an import finishes its file is kept, and
expireBatch the most imports one round handles.
*/
const (
	FileKept    = 30 * 24 * time.Hour
	expireBatch = 200
)

/*
ErrFileExpired refuses the report of an import whose file is gone.
*/
var ErrFileExpired = fault.New(fault.Gone, "import_file_expired",
	"This import's file was deleted 30 days after it finished, so its problem rows can't be downloaded.")

/*
ExpireImports empties old import files and drops changes that can no
longer be undone.
*/
type ExpireImports struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle handles a batch of each and returns how many imports it touched.
*/
func (a *ExpireImports) Handle(ctx context.Context) (int, error) {
	q, now := models.New(a.DB), a.Clock.Now()

	files, err := q.ListImportsToExpire(ctx, models.ListImportsToExpireParams{
		ExpireBefore: now.Add(-FileKept), RowLimit: expireBatch,
	})
	if err != nil {
		return 0, fmt.Errorf("list imports to expire: %w", err)
	}

	for _, f := range files {
		scoped := tenancy.With(ctx, f.WorkspaceID.String())
		if err := writes(scoped, a.DB, func(q *models.Queries) error {
			return q.ExpireImportFile(scoped, models.ExpireImportFileParams{
				Now: now, WorkspaceID: f.WorkspaceID, ID: f.ID,
			})
		}); err != nil {
			return 0, fmt.Errorf("expire import file: %w", err)
		}
	}

	changed, err := q.ListImportChangesToPrune(ctx, models.ListImportChangesToPruneParams{
		PruneBefore: now.Add(-RollbackWindow), RowLimit: expireBatch,
	})
	if err != nil {
		return 0, fmt.Errorf("list import changes to prune: %w", err)
	}

	for _, c := range changed {
		scoped := tenancy.With(ctx, c.WorkspaceID.String())
		if err := writes(scoped, a.DB, func(q *models.Queries) error {
			return q.DeleteImportChanges(scoped, models.DeleteImportChangesParams{
				WorkspaceID: c.WorkspaceID, ImportID: c.ID,
			})
		}); err != nil {
			return 0, fmt.Errorf("prune import changes: %w", err)
		}
	}

	return len(files) + len(changed), nil
}
