/*
|--------------------------------------------------------------------------
| Member Reactivated
|--------------------------------------------------------------------------
|
| A deactivated member got their access back, with the same member id,
| so their earlier history is theirs again. Teams they were taken out of
| are not restored.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
MemberReactivated is dispatched in the transaction that reactivated
the member, with the role they came back with and the member who
brought them back.
*/
type MemberReactivated struct {
	WorkspaceID   ids.UUID
	MemberID      ids.UUID
	UserID        ids.UUID
	Role          string
	ReactivatedBy ids.UUID
}
