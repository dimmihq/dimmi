/*
|--------------------------------------------------------------------------
| AI Conversation Attribute Events
|--------------------------------------------------------------------------
|
| The decision model fills in a workspace's AI conversation attributes
| and checks them again as the customer writes. Each value it sets or
| changes is one event, so workflows can start on it.
|
*/

package events

/*
ConversationAIAttributeChanged is dispatched when the decision model
sets or changes an AI attribute of a conversation: the attribute's key,
the value it held, nil the first time, and the value it holds now.
*/
type ConversationAIAttributeChanged struct {
	Base
	ConversationRef

	Key  string
	From any
	To   any
}

/*
EventName names the event.
*/
func (ConversationAIAttributeChanged) EventName() string { return "conversation.ai_attribute_changed" }
