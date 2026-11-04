/*
|--------------------------------------------------------------------------
| Team Deleted
|--------------------------------------------------------------------------
|
| A team was deleted. It is hidden at once and its name is free again;
| the inbox listens to unassign its conversations and then purge it.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TeamDeleted is dispatched in the transaction that hid the team.
DeletedBy is the member who deleted it.
*/
type TeamDeleted struct {
	WorkspaceID ids.UUID
	TeamID      ids.UUID
	DeletedBy   ids.UUID
}
