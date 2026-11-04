/*
|--------------------------------------------------------------------------
| Channel Outbound
|--------------------------------------------------------------------------
|
| What a channel driver is handed for a reply: the conversation and its
| thread, the body as written in Markdown and also as plain text and
| safe HTML for channels that render one of those, the customer by the
| address the channel reaches them at, and the teammate who wrote it by
| name. The driver never reads the database to learn any of it.
|
*/

package jobs

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
outbound puts together what the driver is handed for the message.
*/
func (w *SendChannelMessage) outbound(ctx context.Context, q *models.Queries, msg models.Message) (outbound, error) {
	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: msg.WorkspaceID, ID: msg.ConversationID,
	})
	if err != nil {
		return outbound{}, fmt.Errorf("find conversation: %w", err)
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: c.WorkspaceID, ID: c.ChannelConnectionID,
	})
	if err != nil {
		return outbound{}, fmt.Errorf("find channel connection: %w", err)
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return outbound{}, fmt.Errorf("find contact: %w", err)
	}

	driver, err := w.Channels.Driver(conn.Type)
	if err != nil {
		return outbound{}, queue.Cancel(err)
	}

	author, err := authorOf(ctx, q, msg)
	if err != nil {
		return outbound{}, err
	}

	message, err := outboundMessage(c, msg, contact)
	message.Author = author

	return outbound{conn: conn, driver: driver, message: message}, err
}

/*
outboundMessage shapes the reply for its driver, its body rendered from
the Markdown it was written in.
*/
func outboundMessage(
	c models.Conversation, msg models.Message, contact models.Contact,
) (contracts.OutboundMessage, error) {
	body := deref(msg.Body)

	rendered, err := markdown.New(markdown.Options{GFM: true}).HTML([]byte(body))
	if err != nil {
		return contracts.OutboundMessage{}, fmt.Errorf("render reply: %w", err)
	}

	return contracts.OutboundMessage{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ConversationNumber: c.Number, MessageID: msg.ID,
		IdempotencyKey: models.PublicID(models.PrefixMessage, msg.ID), ExternalThreadKey: deref(c.ExternalThreadKey),
		Subject: deref(c.Subject), BodyMarkdown: body, BodyText: markdown.PlainText([]byte(body)),
		BodyHTML: rendered.HTML, Recipient: recipientOf(contact),
	}, nil
}

/*
recipientOf names the customer by their email address, the identity
every pushing channel of the product reaches them with.
*/
func recipientOf(contact models.Contact) contracts.Recipient {
	r := contracts.Recipient{ContactID: contact.ID, Name: deref(contact.Name)}
	if contact.Email != nil {
		r.IdentityType, r.IdentityValue = enums.IdentityTypeEmail, *contact.Email
	}

	return r
}

/*
authorOf returns who wrote the reply as the customer sees them, with
the teammate's name when a member wrote it.
*/
func authorOf(ctx context.Context, q *models.Queries, msg models.Message) (contracts.OutboundAuthor, error) {
	author := contracts.OutboundAuthor{Type: enums.AuthorType(msg.AuthorType), ID: orNil(msg.AuthorID)}
	if msg.AuthorID == nil || author.Type != enums.AuthorTypeMember {
		return author, nil
	}

	rows, err := q.MembersByIDs(ctx, models.MembersByIDsParams{
		WorkspaceID: msg.WorkspaceID, Ids: []ids.UUID{*msg.AuthorID},
	})
	if err != nil {
		return author, fmt.Errorf("find reply author: %w", err)
	}

	if len(rows) > 0 {
		author.Name = rows[0].User.Name
	}

	return author, nil
}

/*
deref returns the text, or empty for none.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

/*
orNil returns the id, or ids.Nil() for none.
*/
func orNil(id *ids.UUID) ids.UUID {
	if id == nil {
		return ids.Nil()
	}

	return *id
}
