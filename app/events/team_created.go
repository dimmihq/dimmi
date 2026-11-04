/*
|--------------------------------------------------------------------------
| Team Created
|--------------------------------------------------------------------------
|
| A workspace has a new team that conversations can be routed to.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TeamCreated is dispatched in the transaction that created the team.
CreatedBy is the member who created it.
*/
type TeamCreated struct {
	WorkspaceID ids.UUID
	TeamID      ids.UUID
	Name        string
	CreatedBy   ids.UUID
}
