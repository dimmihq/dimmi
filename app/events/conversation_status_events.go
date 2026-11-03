/*
|--------------------------------------------------------------------------
| Conversation Status Events
|--------------------------------------------------------------------------
|
| A conversation closed, opened again, went to sleep or woke up, and a
| teammate read it. Each is dispatched in the transaction that made the
| change; a change that changed nothing dispatches nothing.
|
*/

package events

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ConversationClosed says a conversation was closed by the member.
*/
type ConversationClosed struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
}

/*
ConversationReopened says a closed conversation opened again, and why:
manual when a teammate reopened it, reply when a reply did,
customer_message when the customer wrote again. ActorID is the member,
or the contact when the customer wrote.
*/
type ConversationReopened struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
	Reason         string
}

/*
ConversationSnoozed says a conversation was put to sleep until a time.
*/
type ConversationSnoozed struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
	Until          time.Time
}

/*
ConversationUnsnoozed says a snoozed conversation woke up, and why:
manual or customer_message.
*/
type ConversationUnsnoozed struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ActorID        ids.UUID
	Reason         string
}

/*
ConversationRead says how far a member has read a conversation, read up
to the message, or marked unread when Unread is set. It concerns that
member alone.
*/
type ConversationRead struct {
	WorkspaceID       ids.UUID
	ConversationID    ids.UUID
	MemberID          ids.UUID
	LastReadMessageID ids.UUID
	Unread            bool
}
