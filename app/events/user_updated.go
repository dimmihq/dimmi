/*
|--------------------------------------------------------------------------
| User Updated
|--------------------------------------------------------------------------
|
| A person changed what teammates see of them: their name, picture or
| address. Everyone in the workspaces they work in hears about it, so open
| screens show the change without a reload.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
UserMembership is one workspace a user is active in, as their member
row there.
*/
type UserMembership struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
}

/*
UserUpdated is dispatched in the transaction that changed the user.
Changes names the fields teammates can see that changed, and Memberships
the workspaces to tell.
*/
type UserUpdated struct {
	UserID      ids.UUID
	Changes     []string
	Memberships []UserMembership
}
