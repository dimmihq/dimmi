/*
|--------------------------------------------------------------------------
| Message Events
|--------------------------------------------------------------------------
|
| What happens on a conversation's timeline: a message or note arrives,
| a note is edited, an entry is deleted, a channel reports how delivery
| went. Message created carries the facts its many listeners need, such
| as billing, notifications and outbound delivery, so none of them has
| to read the message again just to decide whether it cares.
|
| Someone typing is an event too, but one that is never kept: it only
| matters to whoever is watching right now.
|
*/

package events

import (
	fevents "github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
MessageCreated is dispatched when an entry joins a conversation's
timeline. Type is message, note or event; AuthorType and AuthorID say
who wrote it. Substantive tells a real reply from an acknowledgement,
ConversationCreated whether it started the conversation, and
MentionedMemberIDs who a note mentions.
*/
type MessageCreated struct {
	Base
	MessageRef

	ContactID           ids.UUID
	ChannelConnectionID *ids.UUID
	ChannelType         string
	Type                string
	AuthorType          string
	AuthorID            *ids.UUID
	Substantive         bool
	ConversationCreated bool
	MentionedMemberIDs  []ids.UUID
}

/*
EventName names the event.
*/
func (MessageCreated) EventName() string { return "message.created" }

/*
MessageUpdated is dispatched when a note is edited.
AddedMentionedMemberIDs are the members the edit newly mentions.
*/
type MessageUpdated struct {
	Base
	MessageRef

	AddedMentionedMemberIDs []ids.UUID
}

/*
EventName names the event.
*/
func (MessageUpdated) EventName() string { return "message.updated" }

/*
MessageDeleted is dispatched when an entry is deleted.
*/
type MessageDeleted struct {
	Base
	MessageRef
}

/*
EventName names the event.
*/
func (MessageDeleted) EventName() string { return "message.deleted" }

/*
MessageDeliveryUpdated is dispatched when a channel reports a message
sent, delivered, read or failed. Final says no further try follows, and
Reason why it failed, such as canceled for a reply its writer undid.
MessageIDs lists every message of the conversation the report covers, the
one MessageRef names last, when one report covers several, such as a
visitor reading a run of replies.
*/
type MessageDeliveryUpdated struct {
	Base
	MessageRef

	Status     string
	Final      bool
	Reason     string
	MessageIDs []ids.UUID
}

/*
EventName names the event.
*/
func (MessageDeliveryUpdated) EventName() string { return "message.delivery_updated" }

/*
ContactTyping is dispatched while a contact types in a conversation, or
stops. It is never stored: teammates watching the conversation see it,
nobody else ever does. Started is false for a stop.
*/
type ContactTyping struct {
	fevents.Ephemeral
	Base
	ConversationRef

	ContactID ids.UUID
	Started   bool
}

/*
EventName names the event: typing.started or typing.stopped.
*/
func (e ContactTyping) EventName() string {
	if e.Started {
		return "typing.started"
	}

	return "typing.stopped"
}
