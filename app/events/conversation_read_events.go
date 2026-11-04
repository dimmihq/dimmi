/*
|--------------------------------------------------------------------------
| Conversation Read Events
|--------------------------------------------------------------------------
|
| What one member has read, and the counts in their sidebar, are theirs
| alone: these events reach only that member, never the rest of the
| workspace.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ConversationRead is dispatched when a member reads a conversation, or
marks it unread when Unread is set. Only that member hears about it.
*/
type ConversationRead struct {
	Base
	ConversationRef
	MemberRef

	LastReadMessageID *ids.UUID
	Unread            bool
}

/*
EventName names the event.
*/
func (ConversationRead) EventName() string { return "conversation.read" }

/*
ConversationCountsChanged tells one member their view counts changed,
so their sidebar refreshes them.
*/
type ConversationCountsChanged struct {
	Base
	MemberRef
}

/*
EventName names the event.
*/
func (ConversationCountsChanged) EventName() string { return "conversation.counts_changed" }
