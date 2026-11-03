/*
|--------------------------------------------------------------------------
| Message Type
|--------------------------------------------------------------------------
|
| A timeline holds messages exchanged with the customer, internal notes
| the customer never sees, and events such as an assignment. A note
| never leaves the workspace, on any channel.
|
*/

package enums

/*
MessageType is what kind of timeline entry a message is.
*/
type MessageType string

/*
The message types.
*/
const (
	MessageTypeMessage MessageType = "message"
	MessageTypeNote    MessageType = "note"
	MessageTypeEvent   MessageType = "event"
)

/*
MessageTypes returns every type, in the order of the database check.
*/
func MessageTypes() []MessageType {
	return []MessageType{MessageTypeMessage, MessageTypeNote, MessageTypeEvent}
}

/*
Valid reports whether the type is one of the types.
*/
func (t MessageType) Valid() bool {
	switch t {
	case MessageTypeMessage, MessageTypeNote, MessageTypeEvent:
		return true
	}

	return false
}
