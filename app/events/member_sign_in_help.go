/*
|--------------------------------------------------------------------------
| Member Sign-In Help
|--------------------------------------------------------------------------
|
| An admin helped a teammate who cannot sign in: they mailed them a link
| that turns their second factor off, the teammate used it, or the admin
| lifted the lock failed guesses set on the teammate's account. Each is
| part of the workspace's history, so each reaches its audit log.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
MemberTwoFactorResetRequested is dispatched in the transaction that
mailed the reset link to the member. RequestedBy is the admin who asked.
*/
type MemberTwoFactorResetRequested struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	RequestedBy ids.UUID
}

/*
MemberTwoFactorResetCompleted is dispatched in the transaction in which
the member used the link and their second factor was turned off.
*/
type MemberTwoFactorResetCompleted struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
}

/*
MemberSignInLockCleared is dispatched once an admin lifted the sign-in
lock of the member's account. ClearedBy is that admin.
*/
type MemberSignInLockCleared struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	ClearedBy   ids.UUID
}
