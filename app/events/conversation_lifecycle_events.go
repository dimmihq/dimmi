/*
|--------------------------------------------------------------------------
| Conversation Lifecycle Events
|--------------------------------------------------------------------------
|
| A conversation starts, changes subject or inbox, gains or loses
| participants, takes in conversations merged into it, and may end in
| the trash. Each event carries only ids and what changed, never the
| conversation itself, so listeners read it as it is when they run.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ConversationCreated is dispatched when a conversation starts.
InitiatedBy is contact, member, agent, api or import.
*/
type ConversationCreated struct {
	Base
	ConversationRef

	ContactID   ids.UUID
	InitiatedBy string
}

/*
EventName names the event.
*/
func (ConversationCreated) EventName() string { return "conversation.created" }

/*
ConversationUpdated is dispatched when the subject or inbox changes.
Changes maps each changed field to its old and new value. FromInboxID
is the inbox it left, when it moved, for those who could see it only
there.
*/
type ConversationUpdated struct {
	Base
	ConversationRef

	Changes     map[string][2]any
	FromInboxID *ids.UUID `json:"-"`
}

/*
EventName names the event.
*/
func (ConversationUpdated) EventName() string { return "conversation.updated" }

/*
ConversationDeleted is dispatched when a conversation moves to the
trash.
*/
type ConversationDeleted struct {
	Base
	ConversationRef
}

/*
EventName names the event.
*/
func (ConversationDeleted) EventName() string { return "conversation.deleted" }

/*
ConversationMerged is dispatched once per source conversation merged
into the target, which is the conversation the event is about.
*/
type ConversationMerged struct {
	Base
	ConversationRef

	SourceID      ids.UUID
	MovedMessages int
}

/*
EventName names the event.
*/
func (ConversationMerged) EventName() string { return "conversation.merged" }

/*
ConversationParticipantsChanged is dispatched when contacts are added
to or removed from a conversation.
*/
type ConversationParticipantsChanged struct {
	Base
	ConversationRef

	Added   []ids.UUID
	Removed []ids.UUID
}

/*
EventName names the event.
*/
func (ConversationParticipantsChanged) EventName() string { return "conversation.participants_changed" }
