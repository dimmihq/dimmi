/*
|--------------------------------------------------------------------------
| Inbox Events
|--------------------------------------------------------------------------
|
| An inbox was added to a workspace, changed or deleted. Each is
| dispatched in the transaction that made the change.
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
InboxUpdated says an inbox changed; its name went from one to the
other, the same when only its team or settings changed.
*/
type InboxUpdated struct {
	WorkspaceID ids.UUID
	InboxID     ids.UUID
	ActorID     ids.UUID
	FromName    string
	ToName      string
}

/*
InboxDeleted says the member deleted an inbox. MovedTo is the inbox its
conversations went to, nil when it had none. Moved is how many moved.
*/
type InboxDeleted struct {
	WorkspaceID ids.UUID
	InboxID     ids.UUID
	ActorID     ids.UUID
	MovedTo     *ids.UUID
	Moved       int64
}

/*
InboxAccessChanged says the member changed who may open an inbox: Access
is everyone or restricted, and Teams and Members are the inbox's list.
*/
type InboxAccessChanged struct {
	WorkspaceID ids.UUID
	InboxID     ids.UUID
	ActorID     ids.UUID
	Name        string
	Access      string
	Teams       []ids.UUID
	Members     []ids.UUID
}
