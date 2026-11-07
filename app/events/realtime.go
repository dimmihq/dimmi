/*
|--------------------------------------------------------------------------
| Realtime Events
|--------------------------------------------------------------------------
|
| Every inbox event carries the same header: its own id, so clients that
| receive it twice keep it once; the workspace it happened in; when it
| happened; who did it; and the id the client gave the change that caused
| it, so the screen that made the change recognizes its own echo instead
| of applying it twice.
|
| An event names itself in the dotted, past-tense form clients and
| webhooks see, and says which conversation or member it is about. The
| realtime broadcaster routes events by those answers alone, so a new
| event reaches open screens by implementing them, with no change to the
| broadcaster.
|
*/

package events

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Base is the header of every inbox event. ID is the event's own id,
WorkspaceID the workspace it happened in and OccurredAt when. Actor is
who caused it, and ClientMutationID the id the client sent with the
change in the Dimmi-Client-Mutation-Id header, empty when it sent none.
WorkflowChain lists the workflows whose steps led to the change, so a
workflow never starts again from its own changes.
*/
type Base struct {
	ID               ids.UUID
	WorkspaceID      ids.UUID
	OccurredAt       time.Time
	Actor            Actor
	ClientMutationID string
	WorkflowChain    []ids.UUID
}

/*
Actor is who caused an event: Type is member, contact, agent, system or
integration, and ID the row of that kind, ids.Nil() for the system.
*/
type Actor struct {
	Type string
	ID   ids.UUID
}

/*
Header returns the event's header.
*/
func (b Base) Header() Base {
	return b
}

/*
Named is an event that reaches open screens: its dotted name and its
header.
*/
type Named interface {
	/*
		EventName returns the name clients see, such as
		conversation.assigned.
	*/
	EventName() string

	/*
		Header returns the event's header.
	*/
	Header() Base
}

/*
ConversationEvent is an event about one conversation, which every
member viewing the workspace's lists or the conversation hears about.
*/
type ConversationEvent interface {
	Named

	/*
		Conversation returns the conversation the event is about.
	*/
	Conversation() ids.UUID
}

/*
MessageEvent is an event about one message of a conversation.
*/
type MessageEvent interface {
	ConversationEvent

	/*
		Message returns the message the event is about.
	*/
	Message() ids.UUID
}

/*
MemberEvent is an event only one member hears about, such as their own
read state or unread counts.
*/
type MemberEvent interface {
	Named

	/*
		Member returns the member the event is for.
	*/
	Member() ids.UUID
}

/*
ConversationRef names the conversation an event is about. Embed it in
the event's struct.
*/
type ConversationRef struct {
	ConversationID ids.UUID
}

/*
Conversation returns the conversation the event is about.
*/
func (r ConversationRef) Conversation() ids.UUID {
	return r.ConversationID
}

/*
MessageRef names the message an event is about and its conversation.
Embed it in the event's struct.
*/
type MessageRef struct {
	ConversationRef

	MessageID ids.UUID
}

/*
Message returns the message the event is about.
*/
func (r MessageRef) Message() ids.UUID {
	return r.MessageID
}

/*
MemberRef names the one member an event is for. Embed it in the
event's struct.
*/
type MemberRef struct {
	MemberID ids.UUID
}

/*
Member returns the member the event is for.
*/
func (r MemberRef) Member() ids.UUID {
	return r.MemberID
}
