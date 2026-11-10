/*
|--------------------------------------------------------------------------
| Conversations A Broadcast Starts
|--------------------------------------------------------------------------
|
| Every conversation a broadcast starts begins with the broadcast itself,
| written by the teammate who sent it, so whoever picks up a reply sees
| what the person is answering. The conversation names the broadcast and
| the recipient in its first message. A conversation the Messenger
| delivers opens closed, so ten thousand recipients do not flood the
| inbox; a reply reopens it like any other. When the broadcast leaves
| replies to the team, the AI agent stays out.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
opening is a conversation about to start from a broadcast: the
broadcast and recipient, the contact and channel it lives on, the
message as the recipient got it, its status and how the channel
threads it.
*/
type opening struct {
	row     models.Broadcast
	r       models.BroadcastRecipient
	contact models.Contact
	conn    models.ChannelConnection
	body    string
	status  string
	thread  *string
	subject *string
	widget  string
}

/*
open writes the conversation with the broadcast as its first message.
*/
func open(ctx context.Context, tx *database.Tx, svc conversations.Services, o opening) (opened, error) {
	q := models.New(tx)
	ws := o.row.WorkspaceID

	if o.row.SenderMemberID == nil {
		return opened{}, ErrNoSender
	}

	m, ok, err := slackteam.Membership(ctx, q, ws, *o.row.SenderMemberID)
	if err != nil || !ok {
		return opened{}, orSender(err)
	}

	w, err := conversations.NewWriter(ctx, tx, svc, m)
	if err != nil {
		return opened{}, err
	}

	if o.status == string(enums.ConversationStatusOpen) {
		sentAt(w, o.r)
	}

	number, err := q.AllocateConversationNumber(ctx, ws)
	if err != nil {
		return opened{}, fmt.Errorf("allocate conversation number: %w", err)
	}

	c, err := q.CreateConversation(ctx, models.CreateConversationParams{
		WorkspaceID: ws, Number: number, InboxID: o.conn.InboxID, ChannelConnectionID: o.conn.ID,
		ContactID: o.contact.ID, Subject: o.subject, Status: o.status, ExternalThreadKey: o.thread,
		Source: sourceOf(o), Now: w.Now,
	})
	if err != nil {
		return opened{}, fmt.Errorf("create conversation: %w", err)
	}

	if c.Status == string(enums.ConversationStatusClosed) {
		if c, err = q.StampBroadcastConversationClosed(ctx, models.StampBroadcastConversationClosedParams{
			WorkspaceID: ws, ID: c.ID, Now: w.Now,
		}); err != nil {
			return opened{}, fmt.Errorf("date conversation close: %w", err)
		}
	}

	out := opened{w: w}
	out.message, out.conversation, err = firstMessage(ctx, w, c, o)

	return out, err
}

/*
opened is a conversation a broadcast started: the writer that started
it, the conversation and the broadcast's message in it.
*/
type opened struct {
	w            *conversations.Writer
	conversation models.Conversation
	message      models.Message
}

/*
firstMessage writes the broadcast into the conversation as already
delivered, and keeps the AI agent out when replies go to the team.
*/
func firstMessage(ctx context.Context, w *conversations.Writer, c models.Conversation, o opening,
) (models.Message, models.Conversation, error) {
	status := string(enums.DeliveryStatusSent)

	msg, c, err := w.Append(ctx, c, conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: o.body, BodyFormat: new("markdown"),
		DeliveryStatus: &status, Metadata: metadataOf(o), SentAt: &w.Now,
	})
	if err != nil || o.row.AiReplies {
		return msg, c, err
	}

	err = w.Q.SetConversationAIOff(ctx, models.SetConversationAIOffParams{
		WorkspaceID: c.WorkspaceID, ID: c.ID, Now: w.Now,
	})

	return msg, c, wrap("keep agent out", err)
}

/*
metadataOf is what the first message keeps about the broadcast, for the
Messenger, which shows a post as a card, and for the inbox.
*/
func metadataOf(o opening) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"broadcast": map[string]any{
		"id":           models.PublicID(models.PrefixBroadcast, o.row.ID),
		"recipient_id": models.PublicID(models.PrefixBroadcastRecipient, o.r.ID),
		"name":         o.row.Name, "channel": o.row.Channel, "display": displayOf(o.row.Channel),
		"subject": o.row.Subject,
	}})

	return raw
}

/*
sourceOf records on the conversation that a broadcast started it, and
for the Messenger which widget shows it.
*/
func sourceOf(o opening) json.RawMessage {
	source := map[string]any{
		"type": "broadcast", "broadcast_id": models.PublicID(models.PrefixBroadcast, o.row.ID),
	}
	if o.widget != "" {
		source["widget_id"] = o.widget
	}

	raw, _ := json.Marshal(source)

	return raw
}

/*
displayOf is how the Messenger shows a broadcast message.
*/
func displayOf(channel string) string {
	if channel == ChannelPost {
		return "post"
	}

	return "message"
}

/*
orSender is the error for a sender who no longer works in the workspace.
*/
func orSender(err error) error {
	if err != nil {
		return err
	}

	return ErrNoSender
}
