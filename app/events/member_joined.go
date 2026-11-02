/*
|--------------------------------------------------------------------------
| Member Joined
|--------------------------------------------------------------------------
|
| A user became a member of a workspace, by creating it, by invitation,
| through setup or from the command line. JoinedVia says which.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
MemberJoined is dispatched in the transaction that added the member.
*/
type MemberJoined struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	JoinedVia   string
}
