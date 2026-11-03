/*
|--------------------------------------------------------------------------
| Leave Workspace
|--------------------------------------------------------------------------
|
| Anyone may leave a workspace, the same way an admin would deactivate
| them, except the last active owner, who must hand ownership over
| first so the workspace is never left without someone to run it.
| Leaving twice changes nothing. Only the user's own memberships can be
| left; any other id is simply not found.
|
*/

package members

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
LeaveWorkspaceInput is the user and the membership they leave.
*/
type LeaveWorkspaceInput struct {
	UserID   ids.UUID
	MemberID ids.UUID
}

/*
LeaveWorkspace deactivates the user's own membership.

Rules:
  - Only the user's own memberships are found.
  - A membership already deactivated is left alone.
  - The last active owner cannot leave.

Events: [appevents.MemberDeactivated], by the member themselves.

Errors: [exceptions.ErrNotFound], [exceptions.ErrLastOwner], or a
wrapped database error.
*/
type LeaveWorkspace struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle leaves the workspace.
*/
func (a *LeaveWorkspace) Handle(ctx context.Context, in LeaveWorkspaceInput) error {
	own, err := models.New(a.DB).FindUserMembership(ctx, models.FindUserMembershipParams{
		UserID: in.UserID, MemberID: in.MemberID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("find own membership: %w", err)
	}

	if own.Status != string(enums.MemberStatusActive) {
		return nil
	}

	scoped := tenancy.With(ctx, own.WorkspaceID.String())

	return a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if _, err := q.LockWorkspace(ctx, own.WorkspaceID); err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		member, err := q.LockWorkspaceMember(ctx, models.LockWorkspaceMemberParams{
			WorkspaceID: own.WorkspaceID, ID: own.MemberID,
		})
		if err != nil {
			return fmt.Errorf("lock own membership: %w", err)
		}

		if member.Status != string(enums.MemberStatusActive) {
			return nil
		}

		w := memberWriter{tx: tx, events: a.Events, by: member.ID, now: a.Clock.Now()}

		err = w.deactivate(ctx, member, ids.Nil())
		if err != nil || !appauth.IsOwnerRole(enums.Role(member.Role)) {
			return err
		}

		return w.keepsOwner(ctx, member.WorkspaceID)
	})
}
