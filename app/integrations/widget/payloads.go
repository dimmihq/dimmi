/*
|--------------------------------------------------------------------------
| Widget Payloads
|--------------------------------------------------------------------------
|
| Everything a visitor's stream receives is built here, from what the
| visitor may see and nothing else: a message the way the messenger lists
| it, a conversation without its assignment or flags, never a note. Every
| conversation's broadcast names the widget it started on, so a visitor
| using two widgets shows each thread only where it belongs. A payload too
| large to broadcast shrinks to a reference the messenger fetches.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxPayloadBytes is the largest payload broadcast whole.
*/
const maxPayloadBytes = 10 << 10

/*
MessagePayload is a message broadcast to its visitor.
*/
type MessagePayload struct {
	ConversationID string                  `json:"conversation_id"`
	WidgetID       *string                 `json:"widget_id"`
	CustomerStatus string                  `json:"customer_status"`
	Message        resources.WidgetMessage `json:"message"`
}

/*
ConversationPayload is a conversation broadcast to its visitor.
*/
type ConversationPayload struct {
	ConversationID string                       `json:"conversation_id"`
	WidgetID       *string                      `json:"widget_id"`
	Conversation   resources.WidgetConversation `json:"conversation"`
}

/*
ReferencePayload stands in for a payload too large to broadcast: the
messenger fetches what it names.
*/
type ReferencePayload struct {
	ID             string    `json:"id"`
	Object         string    `json:"object"`
	ConversationID string    `json:"conversation_id"`
	WidgetID       *string   `json:"widget_id"`
	UpdatedAt      time.Time `json:"updated_at"`
}

/*
Built is a broadcast ready to send: its payload and the conversation it is
about. Found is false when nothing is sent, such as for a note.
*/
type Built struct {
	Payload      any
	Conversation models.Conversation
	Found        bool
}

/*
loaded is a conversation, its message when one was named, and whether a
messenger lists it.
*/
type loaded struct {
	conv   models.Conversation
	msg    models.Message
	listed bool
}

/*
Payloads builds what visitors' streams receive.
*/
type Payloads struct {
	DB   *database.DB
	Read *actions.ListWidgetConversations
}

/*
Message returns the message's payload, false for anything a visitor never
sees, such as a note.
*/
func (p Payloads) Message(ctx context.Context, ws, conversation, message ids.UUID) (Built, error) {
	l, err := p.load(ctx, ws, conversation, message)
	conv, msg := l.conv, l.msg

	if err != nil || !l.listed || msg.Type != string(enums.MessageTypeMessage) {
		return Built{Conversation: conv}, err
	}

	visible, err := p.Read.Present(ctx, ws, []models.Message{msg})
	if err != nil {
		return Built{Conversation: conv}, err
	}

	widgetID := resources.SourceWidgetID(conv.Source)
	payload := MessagePayload{
		ConversationID: models.PublicID(models.PrefixConversation, conv.ID), WidgetID: widgetID,
		CustomerStatus: resources.CustomerStatus(conv, &visible[0]), Message: resources.NewWidgetMessage(visible[0]),
	}

	return Built{Conversation: conv, Found: true, Payload: fit(payload, ReferencePayload{
		ID: payload.Message.ID, Object: "message", ConversationID: payload.ConversationID, WidgetID: widgetID,
		UpdatedAt: msg.UpdatedAt.UTC(),
	})}, nil
}

/*
Conversation returns the conversation's payload.
*/
func (p Payloads) Conversation(ctx context.Context, ws, conversation ids.UUID) (Built, error) {
	l, err := p.load(ctx, ws, conversation, ids.Nil())
	conv := l.conv

	if err != nil || !l.listed {
		return Built{Conversation: conv}, err
	}

	widgetID := resources.SourceWidgetID(conv.Source)
	payload := ConversationPayload{
		ConversationID: models.PublicID(models.PrefixConversation, conv.ID), WidgetID: widgetID,
		Conversation: resources.NewWidgetConversation(actions.VisibleConversation{Conversation: conv}),
	}

	return Built{Conversation: conv, Found: true, Payload: fit(payload, ReferencePayload{
		ID: payload.ConversationID, Object: "conversation", ConversationID: payload.ConversationID,
		WidgetID: widgetID, UpdatedAt: conv.UpdatedAt.UTC(),
	})}, nil
}

/*
load reads the conversation and, when one is named, its message. Only
conversations a messenger lists are broadcast at all.
*/
func (p Payloads) load(ctx context.Context, ws, conversation, message ids.UUID) (loaded, error) {
	var (
		conv    models.Conversation
		msg     models.Message
		channel string
	)

	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error
		conv, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conversation})
		if err != nil {
			return err
		}

		found, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: ws, ID: conv.ChannelConnectionID,
		})
		if err != nil {
			return err
		}

		channel = found.Type

		if message == ids.Nil() {
			return nil
		}

		msg, err = q.FindMessage(ctx, models.FindMessageParams{
			WorkspaceID: ws, ConversationID: conversation, ID: message,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return loaded{conv: conv, msg: msg}, nil
	}

	if err != nil {
		return loaded{}, fmt.Errorf("load widget broadcast: %w", err)
	}

	listed := channel == string(enums.ChannelTypeWidget) || channel == "email" || channel == "api"

	return loaded{conv: conv, msg: msg, listed: listed && conv.DeletedAt == nil}, nil
}

/*
fit returns the payload, or the reference when the payload is too large.
*/
func fit(payload, reference any) any {
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) > maxPayloadBytes {
		return reference
	}

	return payload
}
