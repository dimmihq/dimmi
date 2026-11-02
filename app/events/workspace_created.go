/*
|--------------------------------------------------------------------------
| Workspace Created
|--------------------------------------------------------------------------
|
| A workspace and its organization exist and have their first owner.
| Setting up a new workspace's defaults happens inside the creating
| transaction, not in listeners, so this event only informs.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
WorkspaceCreated is dispatched in the transaction that created the
workspace.
*/
type WorkspaceCreated struct {
	WorkspaceID ids.UUID
	CreatedBy   ids.UUID
}
