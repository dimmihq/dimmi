/*
|--------------------------------------------------------------------------
| Team Member Removed
|--------------------------------------------------------------------------
|
| A member left a team, taken out by someone or because they lost access
| to the workspace.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TeamMemberRemoved is dispatched once per team left, in the transaction
that removed the member. RemovedBy is the member who removed them, or
who deactivated them.
*/
type TeamMemberRemoved struct {
	WorkspaceID ids.UUID
	TeamID      ids.UUID
	MemberID    ids.UUID
	RemovedBy   ids.UUID
}
