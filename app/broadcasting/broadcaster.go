/*
|--------------------------------------------------------------------------
| Broadcaster
|--------------------------------------------------------------------------
|
| Every inbox event that open screens care about becomes a broadcast,
| once its transaction committed. Where it goes follows from what it is
| about: a change to a conversation updates every list in the workspace
| and the conversation itself, a message lands on its conversation's
| timeline and moves its row in the lists, and an event for one member,
| such as their read state, reaches only that member.
|
| Each broadcast carries the event's id, so a client that receives it
| twice keeps it once, and the client mutation id of the change, so the
| screen that made it drops its own echo. Someone typing is broadcast
| without history, so it is never replayed after a reconnect.
|
| A broadcast is a courtesy to open screens: when one fails, the change
| is still saved and the screens catch up on their next fetch, so the
| failure is logged and nothing is undone.
|
*/

package broadcasting

import (
	"context"
	"errors"
	"fmt"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Publisher publishes messages on channels.
*/
type Publisher interface {
	/*
		Publish publishes the message on every channel.
	*/
	Publish(ctx context.Context, channels []string, m fbroadcasting.Message) error
}

/*
Broadcaster turns domain events into broadcasts.
*/
type Broadcaster struct {
	Server   Publisher
	Payloads Payloads
}

/*
Listen makes the broadcaster hear every event it broadcasts, after the
transaction that dispatched it commits.
*/
func (b *Broadcaster) Listen(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, b.memberEvent)
	events.Listen(d, events.AfterCommit, b.messageEvent)
	events.Listen(d, events.AfterCommit, b.conversationEvent)
	events.Listen(d, events.AfterCommit, b.contactTyping)
	events.Listen(d, events.AfterCommit, b.viewChanged)
	events.Listen(d, events.AfterCommit, b.snoozeListChanged)
	b.listenAccounts(d)
	b.listenRoles(d)
}

/*
memberEvent reaches only the member the event is for.
*/
func (b *Broadcaster) memberEvent(ctx context.Context, e appevents.MemberEvent) error {
	var data any
	if c, ok := e.(appevents.ConversationEvent); ok {
		data = map[string]string{"conversation_id": models.PublicID(models.PrefixConversation, c.Conversation())}
	}

	return b.publish(ctx, []string{MemberChannel(e.Member())}, message(e, data))
}

/*
messageEvent puts the message on its conversation's timeline and moves
the conversation's row in the workspace's lists.
*/
func (b *Broadcaster) messageEvent(ctx context.Context, e appevents.MessageEvent) error {
	h := e.Header()

	entry, found, err := b.Payloads.Message(ctx, h.WorkspaceID, e.Message())
	if err != nil || !found {
		return err
	}

	err = b.publishOrRefer(ctx, ConversationChannel(e.Conversation()), message(e, entry),
		reference(models.PrefixMessage, "message", e.Message(), entry))
	if err != nil {
		return err
	}

	return b.publishRow(ctx, e)
}

/*
conversationEvent updates the workspace's lists and the conversation's
own screen. Message and member events have listeners of their own.
*/
func (b *Broadcaster) conversationEvent(ctx context.Context, e appevents.ConversationEvent) error {
	if !conversationOnly(e) {
		return nil
	}

	if err := b.publishRow(ctx, e); err != nil {
		return err
	}

	h := e.Header()

	full, found, err := b.Payloads.Conversation(ctx, h.WorkspaceID, e.Conversation())
	if err != nil || !found {
		return err
	}

	return b.publishOrRefer(ctx, ConversationChannel(e.Conversation()), message(e, full),
		reference(models.PrefixConversation, "conversation", e.Conversation(), full))
}

/*
contactTyping tells the teammates viewing the conversation that the
contact is typing, without keeping it.
*/
func (b *Broadcaster) contactTyping(ctx context.Context, e appevents.ContactTyping) error {
	m := message(e, map[string]any{
		"conversation_id": models.PublicID(models.PrefixConversation, e.Conversation()),
		"author":          AuthorRef{Type: "contact", ID: new(models.PublicID(models.PrefixContact, e.ContactID))},
	})
	m.Ephemeral = true

	return b.publish(ctx, []string{ConversationChannel(e.Conversation())}, m)
}

/*
publishRow publishes the conversation's list row on the workspace's
channel, under the event's name.
*/
func (b *Broadcaster) publishRow(ctx context.Context, e appevents.ConversationEvent) error {
	h := e.Header()

	row, found, err := b.Payloads.ConversationRow(ctx, h.WorkspaceID, e.Conversation())
	if err != nil || !found {
		return err
	}

	return b.publishOrRefer(ctx, WorkspaceChannel(h.WorkspaceID), message(e, row),
		reference(models.PrefixConversation, "conversation", e.Conversation(), row))
}

/*
publishOrRefer publishes the message, or the reference in its place
when the message is too large to broadcast.
*/
func (b *Broadcaster) publishOrRefer(ctx context.Context, channel string, m fbroadcasting.Message, r Reference) error {
	err := b.publish(ctx, []string{channel}, m)
	if !errors.Is(err, fbroadcasting.ErrPayloadTooLarge) {
		return err
	}

	m.Data = r

	return b.publish(ctx, []string{channel}, m)
}

func (b *Broadcaster) publish(ctx context.Context, channels []string, m fbroadcasting.Message) error {
	if err := b.Server.Publish(ctx, channels, m); err != nil {
		return fmt.Errorf("broadcast %s: %w", m.Event, err)
	}

	return nil
}

/*
conversationOnly reports whether the event is about the conversation as
a whole, rather than one of its messages, one member or someone typing.
*/
func conversationOnly(e appevents.ConversationEvent) bool {
	switch e.(type) {
	case appevents.MessageEvent, appevents.MemberEvent, appevents.ContactTyping:
		return false
	default:
		return true
	}
}

/*
message builds the broadcast of the event with the data: the event's
id, so clients deduplicate, and the client mutation id and actor as
meta.
*/
func message(e appevents.Named, data any) fbroadcasting.Message {
	h := e.Header()

	id := h.ID
	if id == ids.Nil() {
		id = ids.New()
	}

	meta := map[string]string{}
	if h.ClientMutationID != "" {
		meta["client_mutation_id"] = h.ClientMutationID
	}

	if h.Actor.Type != "" {
		meta["actor_type"] = h.Actor.Type
	}

	return fbroadcasting.Message{ID: ids.Encode("evt", id), Event: e.EventName(), Data: data, Meta: meta}
}

/*
reference is what a broadcast carries when its subject is too large.
*/
func reference(prefix models.Prefix, object string, id ids.UUID, subject any) Reference {
	r := Reference{ID: models.PublicID(prefix, id), Object: object}
	if s, ok := subject.(Stamped); ok {
		r.UpdatedAt = s.LastUpdated()
	}

	return r
}
