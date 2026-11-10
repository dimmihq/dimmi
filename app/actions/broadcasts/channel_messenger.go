/*
|--------------------------------------------------------------------------
| Messenger Broadcasts
|--------------------------------------------------------------------------
|
| A chat or post reaches a person as a conversation in the Messenger
| they already use, from the teammate who sent it. It shows at once if
| their Messenger is open and waits unread otherwise; a post shows as
| a card. Their answer is an ordinary message in that conversation.
|
*/

package broadcasts

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MessengerSender delivers chat and post broadcasts in the Messenger.
*/
type MessengerSender struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
}

/*
Ready has nothing to check: who can be reached is decided per person.
*/
func (s *MessengerSender) Ready(context.Context, *models.Queries, models.Broadcast) error {
	return nil
}

/*
Deliver starts the recipient's conversation in the Messenger they used
last.
*/
func (s *MessengerSender) Deliver(ctx context.Context, d Delivery) (Delivered, error) {
	if d.Contact == nil {
		return Delivered{}, &UndeliverableError{Status: "failed", Reason: "contact_removed"}
	}

	var c opened

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		conn, err := q.FindContactWidgetConnection(ctx, models.FindContactWidgetConnectionParams{
			WorkspaceID: d.Recipient.WorkspaceID, ContactID: d.Contact.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return &UndeliverableError{Status: "failed", Reason: ReasonNoMessenger}
		}

		if err != nil {
			return wrap("find messenger", err)
		}

		widget, err := q.FindWidgetSettingsByConnection(ctx, models.FindWidgetSettingsByConnectionParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID,
		})
		if err != nil {
			return wrap("find widget", err)
		}

		subject := optionalText(d.Message.Subject)
		c, err = open(ctx, tx, conversations.Services{Events: s.Events, Clock: s.Clock}, opening{
			row: d.Broadcast, r: d.Recipient, contact: *d.Contact, conn: conn, body: messengerBody(d),
			status: string(enums.ConversationStatusClosed), subject: subject,
			widget: models.PublicID(models.PrefixWidget, widget.ID),
		})

		return err
	})
	if err != nil {
		return Delivered{}, err
	}

	return Delivered{ConversationID: &c.conversation.ID}, nil
}

/*
messengerBody is the message as the Messenger shows it: a post leads
with its title.
*/
func messengerBody(d Delivery) string {
	if d.Broadcast.Channel == ChannelPost && d.Message.Subject != "" {
		return "## " + d.Message.Subject + "\n\n" + d.Message.Markdown
	}

	return d.Message.Markdown
}

/*
optionalText is nil for empty text.
*/
func optionalText(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
