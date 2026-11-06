/*
|--------------------------------------------------------------------------
| Widget Conversation Controller
|--------------------------------------------------------------------------
|
| A visitor's conversations and the messages in them. Every id a visitor
| names is looked up within their own conversations on this widget, so an id
| of anyone else's, in any workspace, is simply not found. A message the
| browser sends twice comes back as it was stored the first time.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/http/useragent"
	"github.com/dimmihq/dimmi/framework/ids"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationController serves a visitor's conversations and messages.
*/
type ConversationController struct {
	Read          *actions.ListWidgetConversations
	Post          *actions.PostWidgetMessage
	Help          *actions.ConfirmAiHelp
	HandoffAction *actions.RequestAiHandoff
	IDs           *ids.Registry
	Cursors       *pagination.Cursors
	Agents        *useragent.Parser
	Clock         clock.Clock
}

/*
Index lists a page of the visitor's conversations, latest activity first;
filter[status] narrows it to open or closed ones.
*/
func (c *ConversationController) Index(ctx context.Context, _ *requests.NoInput) (*ConversationsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{
		Filters:      []string{"status"},
		DefaultLimit: actions.DefaultConversationPage, MaxLimit: actions.MaxConversationPage,
	})
	if err != nil {
		return nil, err
	}

	in := actions.ConversationQuery{Auth: authenticated(ctx), Statuses: query.Filters["status"], Limit: query.Limit}

	if query.Cursor != "" {
		var after actions.Cursor
		if err := c.Cursors.Decode(query.Cursor, &after); err != nil {
			return nil, pagination.ErrInvalidCursor
		}

		in.After = &after
	}

	page, err := c.Read.List(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.WidgetConversation]{Limit: query.Limit}
	for _, item := range page.Items {
		out.Items = append(out.Items, resources.NewWidgetConversation(item))
	}

	if page.Next != nil {
		if out.NextCursor, err = c.Cursors.Encode(page.Next); err != nil {
			return nil, err
		}
	}

	return &ConversationsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Show answers with one of the visitor's conversations.
*/
func (c *ConversationController) Show(
	ctx context.Context, in *requests.ShowWidgetConversationRequest,
) (*ConversationOutput, error) {
	id, err := c.conversationID(in.Conversation)
	if err != nil {
		return nil, err
	}

	found, err := c.Read.Find(ctx, authenticated(ctx), id)
	if err != nil {
		return nil, err
	}

	return &ConversationOutput{Body: fhttp.NewResource(resources.NewWidgetConversation(found))}, nil
}

/*
Store starts a conversation with the visitor's first message.
*/
func (c *ConversationController) Store(
	ctx context.Context, in *requests.StoreWidgetConversationRequest,
) (*StartedConversationOutput, error) {
	posted, message, err := c.post(ctx, nil, in.Body)
	if err != nil {
		return nil, err
	}

	conv, err := c.Read.Find(ctx, authenticated(ctx), posted.Conversation.ID)
	if err != nil {
		return nil, err
	}

	out := &StartedConversationOutput{Status: fhttp.StatusCreated}
	if posted.Duplicate {
		out.Status = fhttp.StatusOK
	}

	out.Body.Data, out.Body.Meta.Message = resources.NewWidgetConversation(conv), message

	return out, nil
}

/*
Messages lists a page of a conversation's messages, oldest first within the
page.
*/
func (c *ConversationController) Messages(
	ctx context.Context, in *requests.IndexWidgetMessagesRequest,
) (*MessagesOutput, error) {
	id, err := c.conversationID(in.Conversation)
	if err != nil {
		return nil, err
	}

	query := actions.MessageQuery{Auth: authenticated(ctx), Conversation: id, Limit: in.Limit}
	if query.Limit == 0 {
		query.Limit = actions.DefaultMessagePage
	}

	if err := c.messageCursors(ctx, in, &query); err != nil {
		return nil, err
	}

	page, err := c.Read.Messages(ctx, query)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.WidgetMessage]{Limit: query.Limit}
	for _, item := range page.Items {
		out.Items = append(out.Items, resources.NewWidgetMessage(item))
	}

	if page.HasMore && in.After == "" && len(page.Items) > 0 {
		oldest := page.Items[0].Message
		if out.NextCursor, err = c.Cursors.Encode(actions.Cursor{At: oldest.CreatedAt, ID: oldest.ID}); err != nil {
			return nil, err
		}
	}

	return &MessagesOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Send writes the visitor's message in the conversation, or in the new one
that takes it when the conversation cannot.
*/
func (c *ConversationController) Send(
	ctx context.Context, in *requests.StoreWidgetMessageRequest,
) (*PostedMessageOutput, error) {
	id, err := c.conversationID(in.Conversation)
	if err != nil {
		return nil, err
	}

	posted, message, err := c.post(ctx, &id, in.Body)
	if err != nil {
		return nil, err
	}

	out := &PostedMessageOutput{Status: fhttp.StatusCreated}
	if posted.Duplicate {
		out.Status = fhttp.StatusOK
	}

	out.Body.Data = message
	out.Body.Meta = PostedMessageMeta{
		ConversationStatus: posted.Conversation.Status, Reopened: posted.Reopened, ServerTime: c.Clock.Now().UTC(),
	}

	if posted.RedirectedFrom != nil {
		from := models.PublicID(models.PrefixConversation, *posted.RedirectedFrom)
		out.Body.Meta.RedirectedFrom = &from
	}

	return out, nil
}

/*
post sends the visitor's message and returns it as they see it.
*/
func (c *ConversationController) post(
	ctx context.Context, conversation *ids.UUID, body requests.WidgetMessageBody,
) (actions.PostedMessage, resources.WidgetMessage, error) {
	r, _ := fhttp.RequestFrom(ctx)
	auth := authenticated(ctx)

	in := actions.MessageInput{
		Auth: auth, Conversation: conversation, ClientID: body.ClientID, Body: body.Body, Context: body.Context,
	}

	if body.Page != nil {
		page := body.Page.Page()
		in.Page = &page
	}

	if r != nil {
		agent := c.Agents.Parse(r.UserAgent())
		in.Browser, in.OS = agent.Browser, agent.OS
	}

	for _, public := range body.AttachmentIDs {
		id, err := c.IDs.Decode(string(models.PrefixAttachment), public)
		if err != nil {
			return actions.PostedMessage{}, resources.WidgetMessage{}, exceptions.ErrAttachmentNotUploaded
		}

		in.AttachmentIDs = append(in.AttachmentIDs, id)
	}

	posted, err := c.Post.Handle(ctx, in)
	if err != nil {
		return posted, resources.WidgetMessage{}, err
	}

	visible, err := c.Read.Present(ctx, auth.Session.WorkspaceID, []models.Message{posted.Message})
	if err != nil {
		return posted, resources.WidgetMessage{}, err
	}

	return posted, resources.NewWidgetMessage(visible[0]), nil
}

/*
messageCursors reads where a page of messages starts.
*/
func (c *ConversationController) messageCursors(
	ctx context.Context, in *requests.IndexWidgetMessagesRequest, query *actions.MessageQuery,
) error {
	if in.Before != "" {
		var before actions.Cursor
		if err := c.Cursors.Decode(in.Before, &before); err != nil {
			return pagination.ErrInvalidCursor
		}

		query.Before = &before
	}

	if in.After == "" {
		return nil
	}

	id, err := c.IDs.Decode(string(models.PrefixMessage), in.After)
	if err != nil {
		return pagination.ErrInvalidCursor
	}

	found, err := c.Read.Message(ctx, query.Auth, query.Conversation, id)
	if err != nil {
		return err
	}

	query.After = &actions.Cursor{At: found.CreatedAt, ID: found.ID}

	return nil
}

/*
conversationID decodes a conversation's public id; anything else is simply
not found.
*/
func (c *ConversationController) conversationID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixConversation), public)
	if err != nil {
		return id, exceptions.ErrWidgetConversationNotFound
	}

	return id, nil
}

/*
AiActionOutput is the no-content answer to a visitor's agent controls.
*/
type AiActionOutput struct {
	Status int
}

/*
Helped confirms the assistant resolved the visitor's conversation.
*/
func (c *ConversationController) Helped(
	ctx context.Context, in *requests.StoreWidgetAiHelpRequest,
) (*AiActionOutput, error) {
	id, err := c.conversationID(in.Conversation)
	if err != nil {
		return nil, err
	}

	if err := c.Help.Handle(ctx, actions.HelpInput{Auth: authenticated(ctx), Conversation: id}); err != nil {
		return nil, err
	}

	return &AiActionOutput{Status: fhttp.StatusNoContent}, nil
}

/*
Handoff hands the visitor's conversation to the team.
*/
func (c *ConversationController) Handoff(
	ctx context.Context, in *requests.StoreWidgetAiHandoffRequest,
) (*AiActionOutput, error) {
	id, err := c.conversationID(in.Conversation)
	if err != nil {
		return nil, err
	}

	if err := c.HandoffAction.Handle(ctx, actions.HandoffInput{Auth: authenticated(ctx), Conversation: id}); err != nil {
		return nil, err
	}

	return &AiActionOutput{Status: fhttp.StatusNoContent}, nil
}
