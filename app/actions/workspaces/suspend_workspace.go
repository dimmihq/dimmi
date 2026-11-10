/*
|--------------------------------------------------------------------------
| Suspend Workspace
|--------------------------------------------------------------------------
|
| The operator of the server can freeze a workspace, for abuse or an
| unpaid bill, and thaw it again. A suspended workspace can still be
| read by its members but nothing in it can be changed, and its mail
| and its chat apps' deliveries wait as they were received; a thaw
| queues the held deliveries for processing. The reason is kept with the workspace so the
| next operator knows why, and both changes are written to the
| workspace's audit log as done by the system at the command line.
|
*/

package workspaces

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SuspendWorkspace stops every change to a workspace, by slug.

Rules:
  - Workspaces in the trash can be suspended too, so a restore does not
    bring back a workspace that was frozen.
  - The reason replaces any earlier one; the time of the first
    suspension stays.
  - The audit log names the system acting at the command line.

Errors: [exceptions.ErrWorkspaceNotFound], or a wrapped error.
*/
type SuspendWorkspace struct {
	DB    *database.DB
	Clock clock.Clock
	Audit *audit.RecordAuditLog
}

/*
Handle suspends the workspace with the slug and returns it as it now
is.
*/
func (a *SuspendWorkspace) Handle(ctx context.Context, slug, reason string) (models.Workspace, error) {
	return operate(ctx, a.DB, slug, func(ctx context.Context, tx *database.Tx, w models.Workspace) (
		models.Workspace, error,
	) {
		q := models.New(tx)

		suspended, err := q.SuspendWorkspace(ctx, models.SuspendWorkspaceParams{
			ID: w.ID, Reason: reason, Now: a.Clock.Now(),
		})
		if err != nil {
			return w, fmt.Errorf("suspend workspace: %w", err)
		}

		return suspended, record(ctx, a.Audit, suspended, audit.Entry{
			Event:   audit.EventWorkspaceSuspended,
			Changes: map[string]any{"suspended_reason": map[string]any{"from": w.SuspendedReason, "to": reason}},
		})
	})
}

/*
UnsuspendWorkspace lets a suspended workspace be changed again, by
slug, and queues the release of the chat app deliveries held while it
was suspended. A workspace that is not suspended is answered as it is,
and no entry is written.

Errors: [exceptions.ErrWorkspaceNotFound], or a wrapped error.
*/
type UnsuspendWorkspace struct {
	DB    *database.DB
	Clock clock.Clock
	Audit *audit.RecordAuditLog
	Queue *queue.Queue
}

/*
Handle unsuspends the workspace with the slug and returns it as it now
is.
*/
func (a *UnsuspendWorkspace) Handle(ctx context.Context, slug string) (models.Workspace, error) {
	return operate(ctx, a.DB, slug, func(ctx context.Context, tx *database.Tx, w models.Workspace) (
		models.Workspace, error,
	) {
		q := models.New(tx)

		if w.SuspendedAt == nil {
			return w, nil
		}

		thawed, err := q.UnsuspendWorkspace(ctx, models.UnsuspendWorkspaceParams{ID: w.ID, Now: a.Clock.Now()})
		if err != nil {
			return w, fmt.Errorf("unsuspend workspace: %w", err)
		}

		if err := a.Queue.PushTx(ctx, tx, ReleaseHeldInboundEventsArgs{WorkspaceID: w.ID}); err != nil {
			return w, fmt.Errorf("queue held deliveries: %w", err)
		}

		return thawed, record(ctx, a.Audit, thawed, audit.Entry{
			Event:   audit.EventWorkspaceUnsuspended,
			Changes: map[string]any{"suspended_reason": map[string]any{"from": w.SuspendedReason, "to": nil}},
		})
	})
}

/*
operate finds the workspace with the slug, the trash included, and runs
the change on it, locked, in a transaction scoped to it.
*/
func operate(
	ctx context.Context, db *database.DB, slug string,
	change func(context.Context, *database.Tx, models.Workspace) (models.Workspace, error),
) (models.Workspace, error) {
	found, err := models.New(db).FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, database.ErrNotFound) {
		return found, exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return found, fmt.Errorf("find workspace %s: %w", slug, err)
	}

	var changed models.Workspace

	err = db.Transaction(tenancy.With(ctx, found.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		w, err := models.New(tx).LockWorkspace(ctx, found.ID)
		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		changed, err = change(ctx, tx, w)

		return err
	})

	return changed, err
}

/*
record writes the operator's change, an entry with its event and
changes, to the workspace's audit log, as the system acting at the
command line.
*/
func record(ctx context.Context, log *audit.RecordAuditLog, w models.Workspace, entry audit.Entry) error {
	entry.WorkspaceID, entry.Via = w.ID, audit.ViaCLI
	entry.Subject = audit.Subject{Type: "workspace", ID: w.ID, Label: w.Name}

	_, err := log.Handle(ctx, entry)

	return err
}
