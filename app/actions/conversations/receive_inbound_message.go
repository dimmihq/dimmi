/*
|--------------------------------------------------------------------------
| Receive Inbound Message
|--------------------------------------------------------------------------
|
| The inbox's side of receiving a message from a channel, as the channel
| contract describes it: find or create the contact, find the
| conversation the message continues or start one, store the message
| and move the conversation along, all in one transaction. The channel
| then writes its own rows in the same transaction through its hook.
| Everything goes through the conversation writer, so a conversation
| a channel starts and a message it brings in dispatch the same events
| as the ones the team writes, and open screens see them at once.
|
| A message the workspace already accepted, by its dedupe key, changes
| nothing. A reply to a conversation closed within the reopen window
| reopens it; a later one starts a new conversation. Automatic mail
| never reopens, wakes or counts as the customer's latest word, and a
| proven address is marked verified on its contact. Mail the provider
| calls spam, or from a blocked contact, is kept without waking the
| conversation, and spam stays with its sender: a new conversation from
| someone whose last one was spam starts as spam.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrConnectionGone means the message arrived on a connection that no
longer exists.
*/
var ErrConnectionGone = errors.New("inbox: channel connection gone")

/*
storedFile is an attachment written to disk before its row exists.
*/
type storedFile struct {
	file contracts.InboundAttachment
	path string
	sum  string
	size int64
}

/*
ReceiveInboundMessage takes inbound messages into the inbox of the
workspace the context is scoped to. Disk holds the attachments, under
DiskName, and ReopenWindow is how long after closing a reply still
reopens a conversation.

Events: [appevents.ConversationCreated], [appevents.MessageCreated], and
the reopening or waking.
*/
type ReceiveInboundMessage struct {
	DB           *database.DB
	Events       *events.Dispatcher
	Clock        clock.Clock
	Disk         filesystem.Filesystem
	DiskName     string
	ReopenWindow time.Duration
	Queue        *queue.Queue
}

/*
ReceiveInboundMessage stores the message in its conversation.
*/
func (r *ReceiveInboundMessage) ReceiveInboundMessage(
	ctx context.Context, msg contracts.InboundMessage,
) (contracts.InboundResult, error) {
	tenant, err := tenancy.Required(ctx)
	if err != nil {
		return contracts.InboundResult{}, err
	}

	workspaceID, err := ids.Parse(tenant)
	if err != nil {
		return contracts.InboundResult{}, fmt.Errorf("read workspace: %w", err)
	}

	files, err := r.storeFiles(ctx, workspaceID, msg.Attachments)
	if err != nil {
		return contracts.InboundResult{}, err
	}

	var result contracts.InboundResult

	err = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w := &Writer{
			Q: models.New(tx), Tx: tx, Events: r.Events, Now: r.Clock.Now(),
			Actor: Actor{Type: "system", ID: ids.Nil()},
		}
		run := &ingestRun{r: r, q: w.Q, w: w, tx: tx, ws: workspaceID, msg: msg, files: files}
		result, err = run.receive(ctx)

		return err
	})

	return result, err
}

/*
ingestRun is one message being received, written through w.
*/
type ingestRun struct {
	r     *ReceiveInboundMessage
	q     *models.Queries
	w     *Writer
	tx    *database.Tx
	ws    ids.UUID
	msg   contracts.InboundMessage
	files []storedFile
}

/*
receive runs the steps of receiving the message.
*/
func (run *ingestRun) receive(ctx context.Context) (contracts.InboundResult, error) {
	conn, err := run.q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: run.ws, ID: run.msg.ConnectionID,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && conn.DeletedAt != nil) {
		return contracts.InboundResult{}, ErrConnectionGone
	}

	if err != nil {
		return contracts.InboundResult{}, fmt.Errorf("find connection: %w", err)
	}

	existing, err := run.q.FindMessageByDedupeKey(ctx, models.FindMessageByDedupeKeyParams{
		WorkspaceID: run.ws, DedupeKey: &run.msg.DedupeKey,
	})
	if err == nil {
		return contracts.InboundResult{
			MessageID: existing.ID, ConversationID: existing.ConversationID, Duplicate: true,
		}, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return contracts.InboundResult{}, fmt.Errorf("find message by dedupe key: %w", err)
	}

	if run.msg.Echo != nil && run.msg.Echo.MemberID != nil {
		return run.echo(ctx, conn)
	}

	contact, err := run.resolveContact(ctx, run.msg.Sender)
	if err != nil {
		return contracts.InboundResult{}, err
	}

	return run.deliver(ctx, conn, contact)
}

/*
deliver stores the message in its conversation as the sender, opens
the conversation again for a person's message, and runs the channel's
hook.
*/
func (run *ingestRun) deliver(
	ctx context.Context, conn models.ChannelConnection, contact models.Contact,
) (contracts.InboundResult, error) {
	sender := run.w.AsContact(contact)

	conv, created, err := run.conversation(ctx, conn, contact)
	if err != nil {
		return contracts.InboundResult{}, err
	}

	if created {
		err := sender.Dispatch(ctx, appevents.ConversationCreated{
			Base: sender.Base(run.ws), ConversationRef: ref(conv), ContactID: conv.ContactID,
			InitiatedBy: string(enums.AuthorTypeContact),
		})
		if err != nil {
			return contracts.InboundResult{}, err
		}
	}

	if created && run.msg.BeforeFirst != nil {
		if err := run.msg.BeforeFirst(ctx, run.tx, conv); err != nil {
			return contracts.InboundResult{}, err
		}
	}

	wake := !run.quiet(contact) && !conv.Spam && conv.Status != string(enums.ConversationStatusOpen)
	result := contracts.InboundResult{
		ContactID: contact.ID, ConversationCreated: created, Spam: conv.Spam,
		Reopened: wake && conv.Status == string(enums.ConversationStatusClosed),
	}

	message, conv, err := run.store(ctx, sender, conn, conv)
	if err != nil {
		return result, err
	}

	result.MessageID, result.ConversationID = message.ID, conv.ID

	return result, run.settle(ctx, landed{sender: sender, conv: conv, contact: contact, message: message, wake: wake})
}

/*
landed is a stored message: who sent it, its conversation and contact,
and whether it wakes the conversation.
*/
type landed struct {
	sender  *Writer
	conv    models.Conversation
	contact models.Contact
	message models.Message
	wake    bool
}

/*
settle opens the conversation again when the message wakes it, hands
it back from an away member who asked for that, puts everyone on the
message on the conversation, records its attachments, and runs the
channel's hook.
*/
func (run *ingestRun) settle(ctx context.Context, l landed) error {
	if l.wake {
		var err error

		l.conv, _, err = l.sender.ChangeStatus(ctx, l.conv, StatusChange{
			Status: string(enums.ConversationStatusOpen), Reason: ReasonCustomerMessage,
		})
		if err != nil {
			return err
		}
	}

	if !run.quiet(l.contact) {
		var err error
		if l.conv, _, err = run.w.handBack(ctx, l.conv); err != nil {
			return err
		}
	}

	if err := run.join(ctx, l.conv, l.contact); err != nil {
		return err
	}

	if err := run.attach(ctx, l.message); err != nil {
		return err
	}

	if run.msg.AfterPersist == nil {
		return nil
	}

	return run.msg.AfterPersist(ctx, run.tx, l.conv, l.message)
}

/*
conversation returns the conversation the message continues, or a new
one, and whether it is new.
*/
func (run *ingestRun) conversation(
	ctx context.Context, conn models.ChannelConnection, contact models.Contact,
) (models.Conversation, bool, error) {
	found, err := run.existing(ctx, conn)
	if err != nil {
		return models.Conversation{}, false, err
	}

	if found != nil && !ClosedBefore(*found, run.r.Clock.Now().Add(-run.r.ReopenWindow)) {
		return *found, false, nil
	}

	spam, err := run.spam(ctx, contact, found)
	if err != nil {
		return models.Conversation{}, false, err
	}

	number, err := run.q.AllocateConversationNumber(ctx, run.ws)
	if err != nil {
		return models.Conversation{}, false, fmt.Errorf("allocate conversation number: %w", err)
	}

	source := json.RawMessage(`{}`)

	if run.msg.Source != nil {
		encoded, err := json.Marshal(run.msg.Source)
		if err != nil {
			return models.Conversation{}, false, fmt.Errorf("encode conversation source: %w", err)
		}

		source = encoded
	}

	subject := optional(run.msg.Subject)
	if _, err := redactText(ctx, run.q, run.ws, subject); err != nil {
		return models.Conversation{}, false, err
	}

	conv, err := run.q.CreateConversation(ctx, models.CreateConversationParams{
		WorkspaceID: run.ws, Number: number, InboxID: conn.InboxID, ChannelConnectionID: conn.ID,
		ContactID: contact.ID, Subject: subject, Status: string(enums.ConversationStatusOpen),
		ExternalThreadKey: optional(run.msg.ExternalThreadKey), Source: source, Now: run.r.Clock.Now(),
	})
	if err != nil {
		return conv, false, fmt.Errorf("create conversation: %w", err)
	}

	conv.Spam = spam

	return run.setAside(ctx, conv)
}

/*
quiet reports whether the message is stored without waking anyone:
automatic mail, mail the provider calls spam, and mail from a blocked
contact.
*/
func (run *ingestRun) quiet(contact models.Contact) bool {
	return run.msg.AutoGenerated || run.msg.SpamVerdict == enums.SpamVerdictSpam || contact.BlockedAt != nil
}

/*
spam reports whether a new conversation starts set aside as spam: when
the provider calls the message spam, the contact is blocked, the thread
it continues was spam, or the contact's latest conversation is spam, so
spam stays with the sender once a teammate marks it.
*/
func (run *ingestRun) spam(ctx context.Context, contact models.Contact, found *models.Conversation) (bool, error) {
	if run.msg.SpamVerdict == enums.SpamVerdictSpam || contact.BlockedAt != nil || (found != nil && found.Spam) {
		return true, nil
	}

	spam, err := run.q.FindLatestContactConversationSpam(ctx, models.FindLatestContactConversationSpamParams{
		WorkspaceID: run.ws, ContactID: contact.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find latest conversation of sender: %w", err)
	}

	return spam, nil
}

/*
setAside stores the spam mark of a new conversation that starts as
spam, and returns it as created.
*/
func (run *ingestRun) setAside(ctx context.Context, conv models.Conversation) (models.Conversation, bool, error) {
	if !conv.Spam {
		return conv, true, nil
	}

	err := run.q.MarkEmailConversationSpam(ctx, models.MarkEmailConversationSpamParams{
		Now: run.r.Clock.Now(), WorkspaceID: run.ws, ID: conv.ID,
	})
	if err != nil {
		return conv, true, fmt.Errorf("mark conversation spam: %w", err)
	}

	return conv, true, nil
}

/*
ClosedBefore reports whether the conversation was closed before the
time, too long ago for a reply to reopen it.
*/
func ClosedBefore(conv models.Conversation, cutoff time.Time) bool {
	return conv.Status == string(enums.ConversationStatusClosed) &&
		(conv.ClosedAt == nil || conv.ClosedAt.Before(cutoff))
}

/*
existing returns the live conversation the channel named, or the newest
one with the message's thread key, or nil.
*/
func (run *ingestRun) existing(ctx context.Context, conn models.ChannelConnection) (*models.Conversation, error) {
	if run.msg.ConversationID != nil {
		conv, err := run.q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: run.ws, ID: *run.msg.ConversationID,
		})
		if err == nil && conv.DeletedAt == nil {
			return &conv, nil
		}

		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("find conversation: %w", err)
		}
	}

	if run.msg.ExternalThreadKey == "" {
		return nil, nil
	}

	conv, err := run.q.FindLatestThreadConversation(ctx, models.FindLatestThreadConversationParams{
		WorkspaceID: run.ws, ChannelConnectionID: conn.ID, ExternalThreadKey: &run.msg.ExternalThreadKey,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find thread conversation: %w", err)
	}

	return &conv, nil
}

/*
store appends the message as its sender, as an internal note by dimmi
when the workspace itself sent it, or as the teammate's note when they
answered by email.
*/
func (run *ingestRun) store(
	ctx context.Context, sender *Writer, conn models.ChannelConnection, conv models.Conversation,
) (models.Message, models.Conversation, error) {
	metadata, err := json.Marshal(map[string]any{"auto_reply": run.msg.AutoGenerated, conn.Type: run.msg.Metadata})
	if err != nil {
		return models.Message{}, conv, fmt.Errorf("encode message metadata: %w", err)
	}

	sentAt := run.msg.SentAt
	kind := enums.MessageTypeMessage

	switch {
	case run.msg.FromWorkspace:
		kind, sender = enums.MessageTypeNote, sender.AsSystem()
	case run.msg.FromMemberID != nil:
		kind, sender = enums.MessageTypeNote, sender.AsMember(*run.msg.FromMemberID, run.msg.FromMemberName)
	}

	return sender.Append(ctx, conv, NewMessage{
		Type: string(kind), Body: run.msg.Body, BodyFormat: optional(string(run.msg.BodyFormat)),
		BodyFull: optional(run.msg.BodyFull), BodyHTML: optional(run.msg.BodyHTML),
		ChannelMessageID: optional(run.msg.ChannelMessageID), DedupeKey: &run.msg.DedupeKey,
		DeliveryStatus: new(string(enums.DeliveryStatusNotApplicable)), Automatic: run.msg.AutoGenerated,
		Metadata: metadata, SentAt: &sentAt,
	})
}

/*
optional returns a pointer to the text, nil when it is empty.
*/
func optional(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}

/*
storeFiles writes the message's attachments to disk before anything
else, so the transaction only records them. They go in the workspace's
folder, which its purge deletes with it.
*/
func (r *ReceiveInboundMessage) storeFiles(
	ctx context.Context, workspaceID ids.UUID, files []contracts.InboundAttachment,
) ([]storedFile, error) {
	stored := make([]storedFile, 0, len(files))

	for _, file := range files {
		content, err := readAll(ctx, file)
		if err != nil {
			return nil, err
		}

		path := jobs.WorkspaceStoragePrefix(workspaceID) + "attachments/" + ids.NewV7At(r.Clock.Now()).String()

		err = r.Disk.Put(ctx, path, content, filesystem.PutOptions{ContentType: file.ContentType})
		if err != nil {
			return nil, fmt.Errorf("store attachment: %w", err)
		}

		stored = append(stored, storedFile{
			file: file, path: path, sum: hashing.SHA256Hex(content), size: int64(len(content)),
		})
	}

	return stored, nil
}

/*
readAll reads an attachment's content.
*/
func readAll(ctx context.Context, file contracts.InboundAttachment) ([]byte, error) {
	reader, err := file.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	defer func() { _ = reader.Close() }()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}

	return content, nil
}

/*
attach records the stored attachments on the message and queues the
malware scan of each. A file is pending until its scan judges it.
*/
func (run *ingestRun) attach(ctx context.Context, message models.Message) error {
	for _, f := range run.files {
		row, err := run.q.CreateEmailAttachment(ctx, models.CreateEmailAttachmentParams{
			WorkspaceID: run.ws, MessageID: &message.ID, UploadedByID: message.AuthorID, Disk: run.r.DiskName,
			Path: f.path, Filename: f.file.Filename, ContentType: f.file.ContentType, SizeBytes: f.size,
			Sha256: &f.sum, Inline: f.file.Inline, ContentID: optional(f.file.ContentID), ScanStatus: "pending",
			Now: run.r.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("record attachment: %w", err)
		}

		err = run.r.Queue.PushTx(ctx, run.tx, jobs.ScanAttachmentArgs{WorkspaceID: run.ws, AttachmentID: row.ID})
		if err != nil {
			return fmt.Errorf("queue attachment scan: %w", err)
		}
	}

	return nil
}
