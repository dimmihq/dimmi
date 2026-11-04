/*
|--------------------------------------------------------------------------
| Team Member Added
|--------------------------------------------------------------------------
|
| A member joined a team and now receives the conversations routed to
| it. Who added them travels along, so a notice to the person can say
| who did it, and is left out when they added themselves.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TeamMemberAdded is dispatched once per member added, in the transaction
that added them. AddedByMemberID is the member who added them.
*/
type TeamMemberAdded struct {
	WorkspaceID     ids.UUID
	TeamID          ids.UUID
	MemberID        ids.UUID
	AddedByMemberID ids.UUID
}
