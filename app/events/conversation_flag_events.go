/*
|--------------------------------------------------------------------------
| Conversation Flag Events
|--------------------------------------------------------------------------
|
| The marks teammates put on a conversation to sort their work: its
| priority, its tags, and whether it is spam. Each change is its own
| event, so a webhook can listen for spam alone without hearing every
| tag.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ConversationPriorityChanged is dispatched when the priority flag flips.
*/
type ConversationPriorityChanged struct {
	Base
	ConversationRef

	Priority bool
}

/*
EventName names the event.
*/
func (ConversationPriorityChanged) EventName() string { return "conversation.priority_changed" }

/*
ConversationTagged is dispatched when tags are added.
*/
type ConversationTagged struct {
	Base
	ConversationRef

	TagIDs []ids.UUID
}

/*
EventName names the event.
*/
func (ConversationTagged) EventName() string { return "conversation.tagged" }

/*
ConversationUntagged is dispatched when tags are removed.
*/
type ConversationUntagged struct {
	Base
	ConversationRef

	TagIDs []ids.UUID
}

/*
EventName names the event.
*/
func (ConversationUntagged) EventName() string { return "conversation.untagged" }

/*
ConversationMarkedSpam is dispatched when a conversation is marked as
spam.
*/
type ConversationMarkedSpam struct {
	Base
	ConversationRef
}

/*
EventName names the event.
*/
func (ConversationMarkedSpam) EventName() string { return "conversation.marked_spam" }

/*
ConversationUnmarkedSpam is dispatched when a conversation is no longer
spam.
*/
type ConversationUnmarkedSpam struct {
	Base
	ConversationRef
}

/*
EventName names the event.
*/
func (ConversationUnmarkedSpam) EventName() string { return "conversation.unmarked_spam" }
