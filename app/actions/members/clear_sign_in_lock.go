/*
|--------------------------------------------------------------------------
| Clear Sign-In Lock
|--------------------------------------------------------------------------
|
| Failed guesses lock an account against strangers for a while, which
| also stops its owner when they are the one who forgot. An admin who
| knows it is really them lifts the lock, so they can try again at once.
| Only the account's own locks go: an address guessing at many accounts
| stays blocked.
|
*/

package members

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ClearSignInLockInput is the admin's membership and the member whose
lock to lift.
*/
type ClearSignInLockInput struct {
	Membership authz.Membership
	MemberID   ids.UUID
}

/*
ClearSignInLock lifts the sign-in lock of a member's account.

Rules:
  - The caller needs members:write.
  - The member belongs to the workspace in play.
  - Clearing an account that is not locked changes nothing but is
    still recorded.

Events: [appevents.MemberSignInLockCleared].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped error.
*/
type ClearSignInLock struct {
	DB      *database.DB
	Events  *events.Dispatcher
	Lockout *appauth.Lockout
}

/*
Handle lifts the lock.
*/
func (a *ClearSignInLock) Handle(ctx context.Context, in ClearSignInLockInput) error {
	if !in.Membership.Can("members:write") {
		return exceptions.ErrForbidden
	}

	target, err := findWithUser(ctx, models.New(a.DB), in.Membership.Workspace.ID, in.MemberID)
	if err != nil {
		return err
	}

	if err := a.Lockout.Clear(ctx, target.User.Email); err != nil {
		return err
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return a.Events.Dispatch(ctx, tx, appevents.MemberSignInLockCleared{
			WorkspaceID: in.Membership.Workspace.ID, MemberID: target.WorkspaceMember.ID, UserID: target.User.ID,
			ClearedBy: in.Membership.Member.ID,
		})
	})
}
