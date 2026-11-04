/*
|--------------------------------------------------------------------------
| Audit Log: Trash
|--------------------------------------------------------------------------
|
| A workspace moved to the trash, and brought back, gets an entry, so
| its owners can see who deleted it and when it would have been purged.
| When the system moved it there rather than a member, the entry names
| the system.
|
*/

package listeners

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listenTrash registers the entries of the workspace's trash.
*/
func (l *WriteAuditLog) listenTrash(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.workspaceDeleted)
	events.Listen(d, events.InTx, l.workspaceRestored)
}

/*
workspaceDeleted records the workspace moved to the trash and when it
will be purged. The event names the user, so the entry finds their
membership.
*/
func (l *WriteAuditLog) workspaceDeleted(ctx context.Context, e appevents.WorkspaceDeleted) error {
	actor := ids.Nil()

	if e.DeletedBy != ids.Nil() {
		member, err := models.New(l.DB).FindAuditActorByUser(ctx, models.FindAuditActorByUserParams{
			WorkspaceID: e.WorkspaceID, UserID: e.DeletedBy,
		})
		if err != nil {
			return fmt.Errorf("find who deleted the workspace: %w", err)
		}

		actor = member.ID
	}

	return l.workspace(ctx, e.WorkspaceID, audit.Entry{
		Event: audit.EventWorkspaceDeleted, ActorID: actor,
		Changes: map[string]any{"purge_after": map[string]any{"to": e.PurgeAfter}},
	})
}

/*
workspaceRestored records the workspace brought back from the trash.
*/
func (l *WriteAuditLog) workspaceRestored(ctx context.Context, e appevents.WorkspaceRestored) error {
	return l.workspace(ctx, e.WorkspaceID, audit.Entry{Event: audit.EventWorkspaceRestored, ActorID: e.RestoredBy})
}

/*
workspace records the entry about the workspace, labeled with its name.
*/
func (l *WriteAuditLog) workspace(ctx context.Context, workspaceID ids.UUID, entry audit.Entry) error {
	w, err := models.New(l.DB).FindWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("find audited workspace: %w", err)
	}

	entry.WorkspaceID = workspaceID
	entry.Subject = audit.Subject{Type: subjectWorkspace, ID: w.ID, Label: w.Name}

	return l.record(ctx, entry)
}
