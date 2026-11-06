/*
|--------------------------------------------------------------------------
| Widget Conversation Activity Controller
|--------------------------------------------------------------------------
|
| What a visitor's messenger reports while a conversation is open: how far
| they read and whether they are typing. Both name a conversation of the
| visitor's own; any other is simply not found.
|
*/

package widget

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReadStateResource is how far the visitor read a conversation.
*/
type ReadStateResource struct {
	Object            string `json:"object"`
	ConversationID    string `json:"conversation_id"`
	LastReadMessageID string `json:"last_read_message_id"`
	UnreadCount       int64  `json:"unread_count"`
}

/*
ReadStateOutput is the visitor's read state.
*/
type ReadStateOutput struct {
	Body fhttp.Resource[ReadStateResource]
}

/*
TypingOutput accepts a typing notice with no body.
*/
type TypingOutput struct {
	Status int
}

/*
ActivityController serves read receipts and typing.
*/
type ActivityController struct {
	Read *actions.MarkWidgetConversationRead
	Tell *actions.BroadcastVisitorTyping
	IDs  *ids.Registry
}

/*
ReadState marks the team's replies up to the message as read.
*/
func (c *ActivityController) ReadState(
	ctx context.Context, in *requests.UpdateWidgetReadStateRequest,
) (*ReadStateOutput, error) {
	conversation, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrWidgetConversationNotFound
	}

	message, err := c.IDs.Decode(string(models.PrefixMessage), in.Body.LastReadMessageID)
	if err != nil {
		return nil, exceptions.ErrReadStateForeignMessage
	}

	state, err := c.Read.Handle(ctx, authenticated(ctx), conversation, message)
	if err != nil {
		return nil, err
	}

	return &ReadStateOutput{Body: fhttp.NewResource(ReadStateResource{
		Object: "widget_read_state", ConversationID: in.Conversation, LastReadMessageID: in.Body.LastReadMessageID,
		UnreadCount: state.UnreadCount,
	})}, nil
}

/*
Typing passes the visitor's typing on to the team.
*/
func (c *ActivityController) Typing(ctx context.Context, in *requests.StoreWidgetTypingRequest) (*TypingOutput, error) {
	conversation, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrWidgetConversationNotFound
	}

	err = c.Tell.Handle(ctx, actions.TypingInput{
		Auth: authenticated(ctx), Conversation: conversation, Started: in.Body.State == "started",
	})
	if err != nil {
		return nil, err
	}

	return &TypingOutput{Status: fhttp.StatusAccepted}, nil
}
