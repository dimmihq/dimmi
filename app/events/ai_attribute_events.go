/*
|--------------------------------------------------------------------------
| Conversation Attribute Events
|--------------------------------------------------------------------------
|
| Teammates, workflows, imports and the AI fill in a conversation's
| attributes. Each value set, changed or cleared is one event, so
| workflows can start on it; the AI's own changes are also the AI
| attribute change workflows started on before.
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

/*
ConversationAttributeChanged is dispatched when a conversation
attribute is set, changed or cleared, by whoever did it: the
attribute's key, the value it held and holds now, nil for none, and By,
who set it (ai, member, workflow or import).
*/
type ConversationAttributeChanged struct {
	Base
	ConversationRef

	Key  string
	From any
	To   any
	By   string
}

/*
EventName names the event.
*/
func (ConversationAttributeChanged) EventName() string { return "conversation.attribute_changed" }
