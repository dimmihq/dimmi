/*
|--------------------------------------------------------------------------
| Channel Outbound
|--------------------------------------------------------------------------
|
| What a channel driver is handed for a reply: the conversation and its
| thread, the body as written in Markdown and also as plain text and
| safe HTML for channels that render one of those, the customer by the
| address the channel reaches them at, everyone else on the thread as a
| copy, the way a reply to all would, and the teammate who wrote it by
| name, with the answer controls an agent's answer offers. The driver
never reads the database to learn any of it. Remote
| pictures leave only when they are GIFs from GIPHY and GIF search is on.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/gifs"
)

/*
errFileNotCleared holds a reply back while one of its files waits on the
malware scan; the job tries again.
*/
var errFileNotCleared = errors.New("reply file not cleared by the malware scan")

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

	message, err := outboundMessage(c, msg, contact, w.remoteImages())
	message.Author, message.Controls, message.Sources = author, controlsOf(msg), sourcesOf(msg)
	if err != nil {
		return outbound{}, err
	}

	if message.CC, err = copiesOf(ctx, q, c); err != nil {
		return outbound{}, err
	}

	message.Attachments, err = w.filesOf(ctx, q, msg)

	return outbound{conn: conn, driver: driver, message: message}, err
}

/*
remoteImages says which remote pictures a reply may carry: GIFs from
GIPHY while GIF search is on, none otherwise.
*/
func (w *SendChannelMessage) remoteImages() func(string) bool {
	if w.GIFs {
		return gifs.Hosted
	}

	return gifs.Never
}

/*
filesOf returns the files a teammate attached to the reply, each opened
from its own disk when the driver reads it. A file the malware scan has
not cleared is held back: the send waits for the scan instead.
*/
func (w *SendChannelMessage) filesOf(
	ctx context.Context, q *models.Queries, msg models.Message,
) ([]contracts.OutboundAttachment, error) {
	rows, err := q.ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
		WorkspaceID: msg.WorkspaceID, MessageIds: []ids.UUID{msg.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("list reply files: %w", err)
	}

	out := make([]contracts.OutboundAttachment, 0, len(rows))
	for _, row := range rows {
		if row.ScanStatus != "clean" && row.ScanStatus != "skipped" {
			return nil, fmt.Errorf("reply file %s is %s: %w", row.ID, row.ScanStatus, errFileNotCleared)
		}

		out = append(out, w.outboundFile(row))
	}

	return out, nil
}

/*
outboundFile names a stored file for a driver and opens it on demand.
*/
func (w *SendChannelMessage) outboundFile(row models.Attachment) contracts.OutboundAttachment {
	return contracts.OutboundAttachment{
		ID: row.ID, Filename: row.Filename, ContentType: row.ContentType, SizeBytes: row.SizeBytes,
		SHA256: deref(row.Sha256), Inline: row.Inline, ContentID: deref(row.ContentID),
		Open: func(ctx context.Context) (io.ReadCloser, error) {
			disk, err := w.Disks.Disk(row.Disk)
			if err != nil {
				return nil, fmt.Errorf("find disk %s: %w", row.Disk, err)
			}

			return disk.Open(ctx, row.Path)
		},
	}
}

/*
copiesOf returns everyone on the conversation besides its customer, by
email, to copy on the reply.
*/
func copiesOf(ctx context.Context, q *models.Queries, c models.Conversation) ([]contracts.Recipient, error) {
	rows, err := q.ListConversationCopies(ctx, models.ListConversationCopiesParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ContactID: c.ContactID,
	})
	if err != nil {
		return nil, fmt.Errorf("list conversation copies: %w", err)
	}

	out := make([]contracts.Recipient, 0, len(rows))
	for _, row := range rows {
		out = append(out, contracts.Recipient{
			ContactID: row.ID, Name: deref(row.Name), IdentityType: enums.IdentityTypeEmail, IdentityValue: row.Email,
		})
	}

	return out, nil
}

/*
outboundMessage shapes the reply for its driver, its body rendered from
the Markdown it was written in, keeping the remote pictures keep allows:
every line break the teammate typed is kept, and the plain version keeps
links' addresses and list numbers, so the customer reads what the thread
shows.
*/
func outboundMessage(
	c models.Conversation, msg models.Message, contact models.Contact, keep func(string) bool,
) (contracts.OutboundMessage, error) {
	body := deref(msg.Body)

	rendered, err := markdown.New(markdown.Options{
		GFM: true, HardWraps: true, RewriteLink: gifs.Images(keep),
	}).HTML([]byte(body))
	if err != nil {
		return contracts.OutboundMessage{}, fmt.Errorf("render reply: %w", err)
	}

	return contracts.OutboundMessage{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ConversationNumber: c.Number, MessageID: msg.ID,
		IdempotencyKey: models.PublicID(models.PrefixMessage, msg.ID), ExternalThreadKey: deref(c.ExternalThreadKey),
		Subject: deref(c.Subject), BodyMarkdown: body, BodyText: markdown.ReadableText([]byte(body)),
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
controlsOf returns the answer controls an agent's answer offers: every
answer gives the customer Solved and Talk to a human, so nobody is left
with an answer they cannot act on; other replies offer what the agent
listed on them, and a teammate's reply offers none.
*/
func controlsOf(msg models.Message) []string {
	if msg.AuthorType != string(enums.AuthorTypeAgent) {
		return nil
	}

	meta := models.ReadAnswerMetadata(msg.Metadata)
	if meta.Outcome == "answered" {
		return []string{contracts.ControlSolved, contracts.ControlHandoff}
	}

	return meta.Actions
}

/*
sourcesOf returns the articles an agent's answer cites.
*/
func sourcesOf(msg models.Message) []contracts.OutboundSource {
	if msg.AuthorType != string(enums.AuthorTypeAgent) {
		return nil
	}

	return contracts.SourcesOf(models.ReadAnswerMetadata(msg.Metadata).Sources)
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
