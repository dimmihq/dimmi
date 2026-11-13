/*
|--------------------------------------------------------------------------
| Open Widget Conversation
|--------------------------------------------------------------------------
|
| A visitor who opens a new chat gets a conversation before they have said
| anything, so a workflow can greet them and offer its answers the moment
| the panel opens, the way a menu in a mobile app is there to tap. The
| conversation starts empty: no message, nobody told, nothing in anyone's
| unread. It carries the same channel, contact and page as one a message
| would have started, so the team sees an ordinary chat once the visitor
| answers.
|
| Only a workspace with a live opener gets one; everywhere else the chat
| still begins with the visitor's first message.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OpenInput is a visitor opening a new chat: the session it comes from and
the page they are on, which the conversation records as its source.
*/
type OpenInput struct {
	Auth    Authenticated
	Page    *Page
	Context map[string]any
	Browser string
	OS      string
}

/*
OpenWidgetConversation starts an empty conversation for a visitor opening
a new chat.

Rules:
  - The visitor's hourly conversation limit and the messenger's start
    rule both apply, as they do to a first message.
  - The conversation starts open, on the widget's channel, authored by
    nobody: it holds no message until the visitor or a workflow writes
    one.

Events: [appevents.ConversationCreated], with Empty set, which is what a
workflow's opener trigger listens for.

Errors: [exceptions.ErrTooManyRequests], [widget.ErrStartRefused], or a
wrapped database error.
*/
type OpenWidgetConversation struct {
	DB       *database.DB
	Clock    clock.Clock
	Events   *events.Dispatcher
	Surfaces *Surfaces
	Limits   *VisitorLimits
}

/*
Handle opens the conversation and returns it.
*/
func (a *OpenWidgetConversation) Handle(ctx context.Context, in OpenInput) (models.Conversation, error) {
	if err := a.mayOpen(ctx, in.Auth); err != nil {
		return models.Conversation{}, err
	}

	var out models.Conversation

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = a.write(ctx, tx, in)

		return err
	})

	return out, err
}

/*
mayOpen refuses a visitor who has opened too many chats this hour, and
one the messenger's start rule does not let start a conversation. It does
not ask for an email: nothing is being sent yet, and the messenger asks
for one when the visitor answers.
*/
func (a *OpenWidgetConversation) mayOpen(ctx context.Context, auth Authenticated) error {
	if err := a.Limits.Conversation(ctx, auth); err != nil {
		return err
	}

	if a.Surfaces == nil {
		return nil
	}

	return a.Surfaces.MayStart(ctx, auth.Resolved.Widget, auth.Session)
}

/*
write stores the conversation and tells everyone it started.
*/
func (a *OpenWidgetConversation) write(
	ctx context.Context, tx *database.Tx, in OpenInput,
) (models.Conversation, error) {
	q, s := models.New(tx), in.Auth.Session
	now := a.Clock.Now()

	w := &conversations.Writer{
		Q: q, Tx: tx, Events: a.Events, Now: now,
		Actor: conversations.Actor{Type: string(enums.AuthorTypeContact), ID: s.ContactID},
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: s.WorkspaceID, ID: in.Auth.Resolved.Widget.ChannelConnectionID,
	})
	if err != nil {
		return models.Conversation{}, fmt.Errorf("find widget channel: %w", err)
	}

	number, err := q.AllocateConversationNumber(ctx, s.WorkspaceID)
	if err != nil {
		return models.Conversation{}, fmt.Errorf("allocate conversation number: %w", err)
	}

	where, err := json.Marshal(source(MessageInput{
		Auth: in.Auth, Page: in.Page, Context: in.Context, Browser: in.Browser, OS: in.OS,
	}, now))
	if err != nil {
		return models.Conversation{}, fmt.Errorf("encode conversation source: %w", err)
	}

	conv, err := q.CreateConversation(ctx, models.CreateConversationParams{
		WorkspaceID: s.WorkspaceID, Number: number, InboxID: conn.InboxID, ChannelConnectionID: conn.ID,
		ContactID: s.ContactID, Status: string(enums.ConversationStatusOpen), Source: where, Now: now,
	})
	if err != nil {
		return conv, fmt.Errorf("create conversation: %w", err)
	}

	if err := q.MarkWidgetConversationStart(ctx, models.MarkWidgetConversationStartParams{
		WorkspaceID: s.WorkspaceID, ID: conv.ID, IsTest: s.Test, Now: now,
	}); err != nil {
		return conv, fmt.Errorf("mark widget conversation start: %w", err)
	}

	return conv, w.Dispatch(ctx, appevents.ConversationCreated{
		Base: w.Base(s.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: conv.ID},
		ContactID: conv.ContactID, InitiatedBy: string(enums.AuthorTypeContact), Empty: true,
	})
}
