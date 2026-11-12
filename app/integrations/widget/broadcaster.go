/*
|--------------------------------------------------------------------------
| Widget Broadcaster
|--------------------------------------------------------------------------
|
| Once a change to a conversation commits, the visitor's open messenger
| hears about it on their contact's channel: a reply, an edit or deletion,
| a read receipt, a closed or reopened thread. A broadcast is a courtesy:
| when one fails the change is still saved, the messenger catches up on its
| next fetch, so the failure is logged and nothing is undone.
|
*/

package widget

import (
	"context"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/http/resources"
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
Broadcaster tells visitors' messengers what changed.
*/
type Broadcaster struct {
	Server   Publisher
	Payloads Payloads
}

/*
DeliveryPayload says how far the team's replies got.
*/
type DeliveryPayload struct {
	ConversationID string   `json:"conversation_id"`
	WidgetID       *string  `json:"widget_id"`
	Status         string   `json:"status"`
	MessageIDs     []string `json:"message_ids"`
}

/*
Listen makes the broadcaster hear what it passes on, after commit.
*/
func (b *Broadcaster) Listen(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MessageCreated) error {
		return b.messageEvent(ctx, e)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MessageUpdated) error {
		return b.messageEvent(ctx, e)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MessageDeleted) error {
		return b.messageEvent(ctx, e)
	})
	events.Listen(d, events.AfterCommit, b.delivery)
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationCreated) error {
		return b.conversation(ctx, e)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationClosed) error {
		return b.conversation(ctx, e)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationReopened) error {
		return b.conversation(ctx, e)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationRatingRequested) error {
		return b.conversation(ctx, e)
	})
}

/*
messageEvent publishes one message's change.
*/
func (b *Broadcaster) messageEvent(ctx context.Context, e appevents.MessageEvent) error {
	h := e.Header()

	built, err := b.Payloads.Message(ctx, h.WorkspaceID, e.Conversation(), e.Message())
	if err != nil || !built.Found {
		return b.logged(ctx, e, err)
	}

	return b.publish(ctx, built.Conversation.ContactID, e, built.Payload)
}

/*
delivery passes on how far the team's replies got.
*/
func (b *Broadcaster) delivery(ctx context.Context, e appevents.MessageDeliveryUpdated) error {
	h := e.Header()

	built, err := b.Payloads.Conversation(ctx, h.WorkspaceID, e.Conversation())
	if err != nil || !built.Found {
		return b.logged(ctx, e, err)
	}

	conv := built.Conversation

	list := e.MessageIDs
	if len(list) == 0 {
		list = []ids.UUID{e.Message()}
	}

	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, models.PublicID(models.PrefixMessage, id))
	}

	return b.publish(ctx, conv.ContactID, e, DeliveryPayload{
		ConversationID: models.PublicID(models.PrefixConversation, conv.ID),
		WidgetID:       resources.SourceWidgetID(conv.Source), Status: e.Status, MessageIDs: out,
	})
}

/*
conversation passes on a conversation that started, closed, reopened or
asks for a rating.
*/
func (b *Broadcaster) conversation(ctx context.Context, e appevents.ConversationEvent) error {
	h := e.Header()

	built, err := b.Payloads.Conversation(ctx, h.WorkspaceID, e.Conversation())
	if err != nil || !built.Found {
		return b.logged(ctx, e, err)
	}

	return b.publish(ctx, built.Conversation.ContactID, e, built.Payload)
}

/*
publish sends the event on the contact's channel, keeping it in history so
a messenger that reconnects in time receives it.
*/
func (b *Broadcaster) publish(ctx context.Context, contact ids.UUID, e appevents.Named, payload any) error {
	m := fbroadcasting.Message{ID: ids.Encode("evt", e.Header().ID), Event: e.EventName(), Data: payload}

	err := b.Server.Publish(ctx, []string{appbroadcasting.WidgetContactChannel(contact)}, m)

	return b.logged(ctx, e, err)
}

/*
logged logs a failed broadcast and lets the change stand.
*/
func (b *Broadcaster) logged(ctx context.Context, e appevents.Named, err error) error {
	if err != nil {
		log.From(ctx).WarnContext(ctx, "broadcast to widget", log.String("event", e.EventName()), log.Err(err))
	}

	return nil
}
