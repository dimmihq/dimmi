/*
|--------------------------------------------------------------------------
| Audit Log Factory
|--------------------------------------------------------------------------
|
| Audit entries for tests, written by the same recorder the product
| uses, at the test clock's current time, so a test moves the clock to
| spread entries over days.
|
*/

package factories

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AuditLog records the entry in its workspace's audit log.
*/
func AuditLog(ctx context.Context, a *app.App, e audit.Entry) (models.AuditLog, error) {
	var entry models.AuditLog

	scoped := tenancy.With(ctx, e.WorkspaceID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, _ *database.Tx) error {
		var err error

		entry, err = a.Actions.RecordAuditLog.Handle(ctx, e)

		return err
	})
	if err != nil {
		return entry, fmt.Errorf("record audit log: %w", err)
	}

	return entry, nil
}
