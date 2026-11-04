/*
|--------------------------------------------------------------------------
| Release Memberships
|--------------------------------------------------------------------------
|
| When the owner of an inbox takes back an account a stranger registered
| with it, whatever the account holds was built by that stranger. A
| workspace the account alone owns goes to the trash: its tokens and
| invitations end, nobody can work in it, and a stranger's other accounts
| lose it too. The real owner is mailed like any owner and may restore it
| during the grace period, which also rescues someone who signed up with
| their own address and came back through a mailed link before ever
| verifying it. Any other membership, including a workspace with another
| active owner, is simply left, so the real owner never finds themselves
| inside a workspace somebody else controls.
|
*/

package members

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReleasedMemberships counts what ReleaseMemberships did: workspaces it
moved to the trash and memberships it ended.
*/
type ReleasedMemberships struct {
	Trashed int
	Left    int
}

/*
ReleaseMemberships lets go of everything a user's account holds, inside
the caller's transaction.

Rules:
  - A live workspace with the user as its only active owner goes to the
    trash, by the system.
  - The user leaves every other workspace they are active in.

Events: those of [workspaces.TrashWorkspace], and
[appevents.MemberDeactivated] by the member themselves.

Errors: a wrapped database or mail error.
*/
type ReleaseMemberships struct {
	Events *events.Dispatcher
	Clock  clock.Clock
	Trash  *workspaces.TrashWorkspace
}

/*
Handle releases the user's memberships, one workspace at a time, and
leaves the transaction scoped to no workspace.
*/
func (a *ReleaseMemberships) Handle(
	ctx context.Context, tx *database.Tx, userID ids.UUID,
) (ReleasedMemberships, error) {
	released := ReleasedMemberships{}

	rows, err := models.New(tx).ListUserWorkspaces(ctx, userID)
	if err != nil {
		return released, fmt.Errorf("list memberships to release: %w", err)
	}

	for _, row := range rows {
		if err := tx.SetTenant(ctx, row.WorkspaceID.String()); err != nil {
			return released, err
		}

		trashed, err := a.release(tenancy.With(ctx, row.WorkspaceID.String()), tx, row.WorkspaceID, row.MemberID)
		if err != nil {
			return released, err
		}

		if trashed {
			released.Trashed++
		} else {
			released.Left++
		}
	}

	return released, tx.SetTenant(ctx, "")
}

/*
release trashes the workspace when the member is its only active owner,
reporting that it did, and otherwise ends the membership.
*/
func (a *ReleaseMemberships) release(
	ctx context.Context, tx *database.Tx, workspaceID, memberID ids.UUID,
) (bool, error) {
	q := models.New(tx)

	s, err := lock(ctx, q, workspaceID, memberID)
	if err != nil {
		return false, err
	}

	owners, err := q.CountActiveOwners(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("count owners to release: %w", err)
	}

	if appauth.IsOwnerRole(enums.Role(s.target.Role)) && owners <= 1 {
		_, err := a.Trash.Handle(ctx, tx, s.workspace, workspaces.TrashedBy{})

		return true, err
	}

	w := memberWriter{tx: tx, events: a.Events, by: memberID, now: a.Clock.Now()}

	return false, w.deactivate(ctx, s.target, ids.Nil())
}
