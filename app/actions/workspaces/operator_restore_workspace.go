/*
|--------------------------------------------------------------------------
| Operator Restore Workspace
|--------------------------------------------------------------------------
|
| The operator of the server can take a workspace out of the trash when
| its owners cannot, such as after a deletion they regret but can no
| longer sign in to undo. The same limits hold as for an owner: only
| until the grace period ends, and never once the purge has begun. The
| owners are mailed, and the audit log names the system as having
| restored it.
|
*/

package workspaces

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
operatorName is how the owners' mail names who restored the workspace.
*/
const operatorName = "System"

/*
ErrWorkspaceNotTrashed refuses to restore or purge a workspace that is
not in the trash.
*/
var ErrWorkspaceNotTrashed = errors.New("workspace is not in the trash")

/*
OperatorRestoreWorkspace takes a workspace out of the trash, by slug,
for the operator of the server.

Rules:
  - The workspace is in the trash.
  - A workspace whose purge began, or whose grace period ended, cannot
    be restored.
  - Every active owner is mailed.

Events: [appevents.WorkspaceRestored], naming no member.

Errors: [exceptions.ErrWorkspaceNotFound], [ErrWorkspaceNotTrashed],
[exceptions.ErrWorkspacePurging], or a wrapped error.
*/
type OperatorRestoreWorkspace struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Mail   *mail.Manager
	Links  appauth.Links
}

/*
Handle restores the workspace with the slug and returns it as it now
is.
*/
func (a *OperatorRestoreWorkspace) Handle(ctx context.Context, slug string) (models.Workspace, error) {
	return operate(ctx, a.DB, slug, func(ctx context.Context, tx *database.Tx, w models.Workspace) (
		models.Workspace, error,
	) {
		q := models.New(tx)

		if w.DeletedAt == nil {
			return w, ErrWorkspaceNotTrashed
		}

		if purgeStarted(w, a.Clock.Now()) {
			return w, exceptions.ErrWorkspacePurging
		}

		restored, err := q.RestoreWorkspace(ctx, models.RestoreWorkspaceParams{ID: w.ID, Now: a.Clock.Now()})
		if err != nil {
			return w, fmt.Errorf("restore workspace: %w", err)
		}

		err = a.Events.Dispatch(ctx, tx, appevents.WorkspaceRestored{WorkspaceID: w.ID, RestoredBy: ids.Nil()})
		if err != nil {
			return restored, err
		}

		owners := RestoreWorkspace{Mail: a.Mail, Links: a.Links}

		return restored, owners.tellOwners(ctx, tx, restored, operatorName)
	})
}
