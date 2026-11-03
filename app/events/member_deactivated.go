/*
|--------------------------------------------------------------------------
| Member Deactivated
|--------------------------------------------------------------------------
|
| A member lost access to the workspace, removed by someone else or by
| leaving. Their history stays theirs; work still assigned to them can
| move to the member named in the reassignment hint.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
MemberDeactivated is dispatched in the transaction that deactivated the
member. DeactivatedBy is the member who did it, the member themselves
when they left, and ReassignTo the member to hand their open work to,
ids.Nil() for none.
*/
type MemberDeactivated struct {
	WorkspaceID   ids.UUID
	MemberID      ids.UUID
	UserID        ids.UUID
	DeactivatedBy ids.UUID
	ReassignTo    ids.UUID
}
