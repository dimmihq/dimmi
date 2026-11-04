/*
|--------------------------------------------------------------------------
| Appending Messages
|--------------------------------------------------------------------------
|
| Every message and note joins a conversation the same way: the row is
| written, a message counts toward the conversation's activity and its
| first and last customer message or reply, a note only marks the
| conversation changed, and the event goes out. Whoever writes something
| has read the conversation up to it.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NewMessage is an entry to append. Type is message or note; the author
is the writer's actor. DeliveryStatus is how the entry starts on its
channel, nil for notes. DedupeKey makes a retried request find its
first entry. Metadata defaults to an empty object. A channel bringing a
message in also gives its HTML, its own id for it and when it was sent.
Automatic marks a message nobody wrote by hand, such as an auto-reply,
which only moves the timeline and never counts as anyone's latest word.
*/
type NewMessage struct {
	Type             string
	Body             string
	BodyHTML         *string
	ChannelMessageID *string
	SentAt           *time.Time
	DeliveryStatus   *string
	DedupeKey        *string
	Metadata         json.RawMessage
	Automatic        bool
}

/*
Append writes the entry as the actor, after the workspace's redaction
rules, and returns it with the conversation as it now is.
*/
func (w *Writer) Append(
	ctx context.Context, c models.Conversation, m NewMessage,
) (models.Message, models.Conversation, error) {
	hits, err := w.redactMessage(ctx, c.WorkspaceID, &m)
	if err != nil {
		return models.Message{}, c, err
	}

	substantive := m.Type == string(enums.MessageTypeMessage) && !m.Automatic

	var via *ids.UUID
	if m.Type == string(enums.MessageTypeMessage) {
		via = &c.ChannelConnectionID
	}

	msg, err := w.Q.CreateMessage(ctx, models.CreateMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Type: m.Type, AuthorType: w.Actor.Type,
		AuthorID: w.authorID(), Body: &m.Body, BodyHtml: m.BodyHTML, ChannelMessageID: m.ChannelMessageID,
		DedupeKey: m.DedupeKey, DeliveryStatus: m.DeliveryStatus, Substantive: substantive, Metadata: m.Metadata,
		SentAt: m.SentAt, ChannelConnectionID: via, Now: w.Now,
	})
	if err != nil {
		return msg, c, fmt.Errorf("add %s: %w", m.Type, err)
	}

	if c, err = w.count(ctx, c, m, msg); err != nil {
		return msg, c, err
	}

	if err := w.announceRedaction(ctx, c, msg, hits); err != nil {
		return msg, c, err
	}

	connection := c.ChannelConnectionID
	if err := w.Dispatch(ctx, appevents.MessageCreated{
		Base:       w.Base(c.WorkspaceID),
		MessageRef: appevents.MessageRef{ConversationRef: ref(c), MessageID: msg.ID},
		ContactID:  c.ContactID, ChannelConnectionID: &connection, Type: msg.Type,
		AuthorType: msg.AuthorType, AuthorID: msg.AuthorID, Substantive: msg.Substantive,
	}); err != nil {
		return msg, c, err
	}

	if w.Actor.Type != string(enums.AuthorTypeMember) {
		return msg, c, nil
	}

	_, err = w.MarkRead(ctx, c, w.Actor.ID, &msg)

	return msg, c, err
}

/*
touch marks the conversation changed, for a note.
*/
func (w *Writer) touch(ctx context.Context, c models.Conversation) (models.Conversation, error) {
	err := w.Q.TouchConversation(ctx, models.TouchConversationParams{Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID})
	if err != nil {
		return c, fmt.Errorf("touch conversation: %w", err)
	}

	return c, nil
}

/*
count counts a message into the conversation's activity, and only marks
the conversation changed for a note.
*/
func (w *Writer) count(
	ctx context.Context, c models.Conversation, m NewMessage, msg models.Message,
) (models.Conversation, error) {
	if m.Type != string(enums.MessageTypeMessage) {
		return w.touch(ctx, c)
	}

	updated, err := w.Q.RecordConversationMessage(ctx, models.RecordConversationMessageParams{
		At: w.Now, FromCustomer: w.Actor.Type == string(enums.AuthorTypeContact), Automatic: m.Automatic,
		MessageID: msg.ID, WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return c, fmt.Errorf("count message: %w", err)
	}

	return updated, nil
}

/*
AddNote adds an internal note as the actor.
*/
func (w *Writer) AddNote(ctx context.Context, c models.Conversation, body string) (models.Message, error) {
	msg, _, err := w.Append(ctx, c, NewMessage{Type: string(enums.MessageTypeNote), Body: body})

	return msg, err
}

/*
AsContact returns a writer acting as the conversation's customer.
*/
func (w *Writer) AsContact(contact models.Contact) *Writer {
	name := ""
	if contact.Name != nil {
		name = *contact.Name
	}

	clone := *w
	clone.Actor = Actor{Type: string(enums.AuthorTypeContact), ID: contact.ID, Name: name}
	clone.Timezone = nil

	return &clone
}
