/*
|--------------------------------------------------------------------------
| Reaction Events
|--------------------------------------------------------------------------
|
| Teammates react to notes with emoji. A change only matters to whoever
| has the conversation open, so it reaches the conversation's screens and
| nothing else.
|
*/

package events

/*
MessageReactionsChanged is dispatched when a teammate adds or takes off
an emoji reaction to a note.
*/
type MessageReactionsChanged struct {
	Base
	MessageRef
}

/*
EventName names the event.
*/
func (MessageReactionsChanged) EventName() string { return "message.reactions_changed" }
