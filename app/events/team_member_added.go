/*
|--------------------------------------------------------------------------
| Team Member Added
|--------------------------------------------------------------------------
|
| A member joined a team and now receives the conversations routed to
| it.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TeamMemberAdded is dispatched once per member added, in the transaction
that added them. AddedBy is the member who added them.
*/
type TeamMemberAdded struct {
	WorkspaceID ids.UUID
	TeamID      ids.UUID
	MemberID    ids.UUID
	AddedBy     ids.UUID
}
