/*
|--------------------------------------------------------------------------
| Mark Widget Conversation Read
|--------------------------------------------------------------------------
|
| A visitor's messenger says how far they read, and the team's replies up to
| there count as read, which the teammate sees on their screen. The pointer
| only moves forward: an older report changes nothing. One report is one
| event, however many replies it covers, and it never says which teammate
| wrote them.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReadState is how far a visitor read a conversation, and how many of the
team's replies still wait unread.
*/
type ReadState struct {
	ConversationID    ids.UUID
	LastReadMessageID ids.UUID
	UnreadCount       int64
}

/*
MarkWidgetConversationRead records how far visitors read.

Events: [appevents.MessageDeliveryUpdated].
*/
type MarkWidgetConversationRead struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle marks the team's replies up to the message as read.
*/
func (a *MarkWidgetConversationRead) Handle(
	ctx context.Context, auth Authenticated, conversation, upTo ids.UUID,
) (ReadState, error) {
	out := ReadState{ConversationID: conversation, LastReadMessageID: upTo}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), auth.Session.WorkspaceID

		conv, err := q.FindWidgetConversation(ctx, scopeOf(auth, conversation))
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrWidgetConversationNotFound
		}

		if err != nil {
			return fmt.Errorf("find widget conversation: %w", err)
		}

		pointer, err := q.FindWidgetMessage(ctx, models.FindWidgetMessageParams{
			WorkspaceID: ws, ConversationID: conversation, ID: upTo,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrReadStateForeignMessage
		}

		if err != nil {
			return fmt.Errorf("find read message: %w", err)
		}

		read, err := q.MarkWidgetMessagesRead(ctx, models.MarkWidgetMessagesReadParams{
			Now: a.Clock.Now(), WorkspaceID: ws, ConversationID: conversation, UpToAt: pointer.CreatedAt, UpToID: upTo,
		})
		if err != nil {
			return fmt.Errorf("mark widget messages read: %w", err)
		}

		out.UnreadCount = conv.UnreadCount - int64(len(read))

		return a.announce(ctx, tx, conv.Conversation, read)
	})

	return out, err
}

/*
announce tells the team, in one event, which replies the visitor read.
*/
func (a *MarkWidgetConversationRead) announce(
	ctx context.Context, tx *database.Tx, conv models.Conversation, read []ids.UUID,
) error {
	if len(read) == 0 {
		return nil
	}

	return a.Events.Dispatch(ctx, tx, appevents.MessageDeliveryUpdated{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: conv.WorkspaceID, OccurredAt: a.Clock.Now(),
			Actor: appevents.Actor{Type: string(enums.AuthorTypeContact), ID: conv.ContactID},
		},
		MessageRef: appevents.MessageRef{
			ConversationRef: appevents.ConversationRef{ConversationID: conv.ID}, MessageID: read[len(read)-1],
		},
		Status: string(enums.DeliveryStatusRead), MessageIDs: read,
	})
}
