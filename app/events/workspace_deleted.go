/*
|--------------------------------------------------------------------------
| Workspace Deleted
|--------------------------------------------------------------------------
|
| A workspace went to the trash. Clients working in it leave, its tokens
| and pending invitations stop working, and it is purged for good once
| its time in the trash runs out unless an owner restores it first.
|
*/

package events

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
WorkspaceDeleted is dispatched in the transaction that trashed the
workspace. DeletedBy is the user who deleted it, ids.Nil() when the
system did, and PurgeAfter when it will be purged.
*/
type WorkspaceDeleted struct {
	WorkspaceID ids.UUID
	DeletedBy   ids.UUID
	PurgeAfter  time.Time
}
