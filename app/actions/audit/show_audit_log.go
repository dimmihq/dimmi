/*
|--------------------------------------------------------------------------
| Show Audit Log
|--------------------------------------------------------------------------
|
| One entry of a workspace's audit log, for following a link to it. An
| entry of another workspace answers as if it did not exist.
|
*/

package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ShowAuditLog returns one entry of the caller's workspace's audit log.

Rules:
  - The caller needs audit_logs:read.
  - Entries of other workspaces are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowAuditLog struct {
	DB *database.DB
}

/*
Handle returns the entry.
*/
func (a *ShowAuditLog) Handle(ctx context.Context, m authz.Membership, id ids.UUID) (models.AuditLog, error) {
	if !m.Can(readPermission) {
		return models.AuditLog{}, exceptions.ErrForbidden
	}

	var entry models.AuditLog

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		entry, err = models.New(tx).FindAuditLog(ctx, models.FindAuditLogParams{WorkspaceID: m.Workspace.ID, ID: id})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return entry, exceptions.ErrNotFound
	}

	if err != nil {
		return entry, fmt.Errorf("find audit log: %w", err)
	}

	return entry, nil
}
