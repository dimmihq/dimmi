/*
|--------------------------------------------------------------------------
| Conversation Events
|--------------------------------------------------------------------------
|
| What happens to a conversation while a team works it: it starts, it is
| handed to someone, and its details or spam flag change. Each is
| dispatched in the transaction that made the change, so a listener
| never hears of a change that did not happen. A change that changed
| nothing dispatches nothing.
|
| Ids that name nobody, such as the member before a first assignment,
| are ids.Nil().
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ConversationCreated says a conversation started. InitiatedBy is who
opened the thread: contact, member or api. ActorType and ActorID name
the author of its first message.
*/
type ConversationCreated struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	InitiatedBy    string
	ActorType      string
	ActorID        ids.UUID
}

/*
ConversationAssigned says who holds a conversation changed: the member,
the team or both, from what to what. ActorID is the member who made the
change.
*/
type ConversationAssigned struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
	FromMemberID   ids.UUID
	ToMemberID     ids.UUID
	FromTeamID     ids.UUID
	ToTeamID       ids.UUID
}

/*
ConversationUpdated says a conversation's subject or inbox changed.
Changes maps each changed field to its value before and after.
*/
type ConversationUpdated struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
	Changes        map[string][2]any
}

/*
ConversationPriorityChanged says a conversation was marked a priority,
or no longer is.
*/
type ConversationPriorityChanged struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
	Priority       bool
}

/*
ConversationSpamChanged says a conversation was marked as spam, or taken
out of spam.
*/
type ConversationSpamChanged struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
	Spam           bool
}
