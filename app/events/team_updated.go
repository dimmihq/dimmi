/*
|--------------------------------------------------------------------------
| Team Updated
|--------------------------------------------------------------------------
|
| A team was renamed. Listeners that show the team read it again rather
| than trusting a copy.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TeamUpdated is dispatched in the transaction that renamed the team.
From and To are the old and new names, and UpdatedBy the member who
renamed it.
*/
type TeamUpdated struct {
	WorkspaceID ids.UUID
	TeamID      ids.UUID
	From        string
	To          string
	UpdatedBy   ids.UUID
}
