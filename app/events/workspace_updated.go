/*
|--------------------------------------------------------------------------
| Workspace Updated
|--------------------------------------------------------------------------
|
| A workspace's name, address, language or time zone changed. Only the
| names of the changed fields travel with it, so listeners that show the
| workspace read it again rather than trusting a copy.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
WorkspaceUpdated is dispatched in the transaction that changed the
workspace. Changes names the fields that changed, sorted, and UpdatedBy
is the member who changed them.
*/
type WorkspaceUpdated struct {
	WorkspaceID ids.UUID
	UpdatedBy   ids.UUID
	Changes     []string
}
