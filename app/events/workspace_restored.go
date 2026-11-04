/*
|--------------------------------------------------------------------------
| Workspace Restored
|--------------------------------------------------------------------------
|
| An owner took a workspace out of the trash before it was purged. Its
| members are back in; the tokens and invitations ended by the deletion
| stay ended.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
WorkspaceRestored is dispatched in the transaction that restored the
workspace. RestoredBy is the member who restored it.
*/
type WorkspaceRestored struct {
	WorkspaceID ids.UUID
	RestoredBy  ids.UUID
}
