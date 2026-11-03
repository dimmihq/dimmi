/*
|--------------------------------------------------------------------------
| Member Role Changed
|--------------------------------------------------------------------------
|
| A member's role changed, so what they may do in the workspace changed
| with it. Handing ownership over changes two roles and fires this once
| for each member.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
MemberRoleChanged is dispatched in the transaction that changed the
role. From and To are the old and new roles, and ChangedBy the member
who changed it.
*/
type MemberRoleChanged struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	From        string
	To          string
	ChangedBy   ids.UUID
}
