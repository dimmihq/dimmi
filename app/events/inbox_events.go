/*
|--------------------------------------------------------------------------
| Inbox Events
|--------------------------------------------------------------------------
|
| An inbox was added to a workspace, or renamed. Each is dispatched in
| the transaction that made the change.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
InboxCreated says an inbox was added by the member.
*/
type InboxCreated struct {
	WorkspaceID ids.UUID
	InboxID     ids.UUID
	ActorID     ids.UUID
}

/*
InboxUpdated says an inbox's name changed, from one to the other.
*/
type InboxUpdated struct {
	WorkspaceID ids.UUID
	InboxID     ids.UUID
	ActorID     ids.UUID
	FromName    string
	ToName      string
}
