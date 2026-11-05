/*
|--------------------------------------------------------------------------
| Purge Trashed Workspace
|--------------------------------------------------------------------------
|
| The operator of the server can delete a workspace for good at once
| rather than wait out its grace period, such as when its owners ask for
| their data to be gone now. Only a workspace already in the trash can
| be purged this way, so a live workspace is never one command away from
| being destroyed. The purge itself is the daily one, retention guard
| included: the grace period is ended now, and the purge runs to the
| end while the operator waits.
|
*/

package workspaces

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrWorkspaceHeld means the retention guard keeps the workspace, so it
stays in the trash.
*/
var ErrWorkspaceHeld = errors.New("workspace is held by the retention guard")

/*
Purger deletes a workspace that is due for purging, and reports whether
it is gone.
*/
type Purger interface {
	/*
		Purge deletes the workspace for good, unless it is not due or the
		retention guard holds it.
	*/
	Purge(ctx context.Context, id ids.UUID) (bool, error)
}

/*
PurgeTrashedWorkspace deletes a workspace in the trash for good, by
slug, without waiting for its grace period to end.

Rules:
  - Only a workspace in the trash is purged.
  - The retention guard is asked first, as for every purge.

Errors: [exceptions.ErrWorkspaceNotFound], [ErrWorkspaceNotTrashed],
[ErrWorkspaceHeld], or a wrapped error.
*/
type PurgeTrashedWorkspace struct {
	DB     *database.DB
	Clock  clock.Clock
	Purger Purger
}

/*
Handle ends the workspace's grace period and purges it, returning it as
it was.
*/
func (a *PurgeTrashedWorkspace) Handle(ctx context.Context, slug string) (models.Workspace, error) {
	w, err := operate(ctx, a.DB, slug, func(ctx context.Context, tx *database.Tx, w models.Workspace) (
		models.Workspace, error,
	) {
		if w.DeletedAt == nil {
			return w, ErrWorkspaceNotTrashed
		}

		ended, err := models.New(tx).EndWorkspaceGrace(ctx, models.EndWorkspaceGraceParams{
			ID: w.ID, Now: a.Clock.Now(),
		})
		if err != nil {
			return w, fmt.Errorf("end grace period: %w", err)
		}

		return ended, nil
	})
	if err != nil {
		return w, err
	}

	purged, err := a.Purger.Purge(ctx, w.ID)
	if err != nil {
		return w, fmt.Errorf("purge workspace %s: %w", slug, err)
	}

	if !purged {
		return w, ErrWorkspaceHeld
	}

	return w, nil
}
