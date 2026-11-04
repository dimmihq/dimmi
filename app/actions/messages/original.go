/*
|--------------------------------------------------------------------------
| Original Messages
|--------------------------------------------------------------------------
|
| The thread shows what a customer wrote this time. This is the rest of
| what they sent: the whole text and the sanitized HTML, quoted thread
| and all, for anyone who may read the conversation. The raw email as it
| arrived holds what the sanitizer took out, hidden text and headers
| included, so it is only for those allowed to read raw mail, the same as
| the list of received mail. A deleted message has no original to show,
| and retention may have purged the raw file long before the message.
|
*/

package messages

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
rawMailPermission lets a member read mail as it arrived.
*/
const rawMailPermission = "email:read_raw"

/*
OriginalInput is who asks for which message of which conversation.
*/
type OriginalInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	MessageID  ids.UUID
}

/*
Original is the whole of a received message.

HTML is safe to show in a sandboxed frame and loads nothing from
elsewhere, and is empty when the mail had none or it was too large to
keep. RawAvailable says the caller may download the raw email and it
still exists.
*/
type Original struct {
	Message      models.Message
	Text         string
	HTML         string
	Limits       models.EmailMetadata
	RawAvailable bool
}

/*
ShowOriginal reads the whole of a received message.

Rules:
  - The caller needs conversations:read.
  - Only a message that came in by email has an original, and a deleted
    one has none any more.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowOriginal struct {
	DB *database.DB
}

/*
Handle returns the original.
*/
func (a *ShowOriginal) Handle(ctx context.Context, in OriginalInput) (Original, error) {
	if err := conversations.CanRead(in.Membership); err != nil {
		return Original{}, err
	}

	var out Original

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		m, err := findOriginal(ctx, q, in)
		if err != nil {
			return err
		}

		out = Original{Message: m, Text: *m.BodyFull, Limits: models.ReadEmailMetadata(m.Metadata)}
		if m.BodyHtml != nil {
			out.HTML = inbound.ReadableHTML(*m.BodyHtml)
		}

		if !in.Membership.Can(rawMailPermission) {
			return nil
		}

		received, err := q.FindMessageInboundEmail(ctx, models.FindMessageInboundEmailParams{
			WorkspaceID: m.WorkspaceID, MessageID: &m.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find received email: %w", err)
		}

		out.RawAvailable = received.RawPath != nil

		return nil
	})

	return out, err
}

/*
RawEmail is a received email as it arrived, and the name to save it
under.
*/
type RawEmail struct {
	Filename string
	Content  []byte
}

/*
DownloadOriginal reads a received message's raw email.

Rules:
  - The caller needs conversations:read and email:read_raw.
  - The message must have come in by email, not be deleted, and its raw
    file must not have been purged.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database or storage error.
*/
type DownloadOriginal struct {
	DB      *database.DB
	Storage *filesystem.Manager
}

/*
Handle returns the raw email.
*/
func (a *DownloadOriginal) Handle(ctx context.Context, in OriginalInput) (RawEmail, error) {
	if err := conversations.CanRead(in.Membership); err != nil {
		return RawEmail{}, err
	}

	if !in.Membership.Can(rawMailPermission) {
		return RawEmail{}, exceptions.ErrForbidden
	}

	var received models.InboundEmail

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		m, err := findOriginal(ctx, q, in)
		if err != nil {
			return err
		}

		received, err = q.FindMessageInboundEmail(ctx, models.FindMessageInboundEmailParams{
			WorkspaceID: m.WorkspaceID, MessageID: &m.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find received email: %w", err)
		}

		return nil
	})
	if err != nil {
		return RawEmail{}, err
	}

	content, err := a.read(ctx, received)
	if err != nil {
		return RawEmail{}, err
	}

	return RawEmail{Filename: models.PublicID(models.PrefixMessage, in.MessageID) + ".eml", Content: content}, nil
}

/*
read loads the raw file, which retention may have purged.
*/
func (a *DownloadOriginal) read(ctx context.Context, received models.InboundEmail) ([]byte, error) {
	if received.RawPath == nil {
		return nil, exceptions.ErrNotFound
	}

	disk, err := a.Storage.Disk(received.RawDisk)
	if err != nil {
		return nil, fmt.Errorf("open raw disk: %w", err)
	}

	content, err := disk.Get(ctx, *received.RawPath)
	if errors.Is(err, filesystem.ErrNotFound) {
		return nil, exceptions.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read raw email: %w", err)
	}

	return content, nil
}

/*
findOriginal finds the message in the conversation, which must be one
in the caller's workspace, when it has an original to show.
*/
func findOriginal(ctx context.Context, q *models.Queries, in OriginalInput) (models.Message, error) {
	c, err := conversations.Find(ctx, q, in.Membership.Workspace.ID, in.Ref)
	if err != nil {
		return models.Message{}, err
	}

	m, err := q.FindMessage(ctx, models.FindMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ID: in.MessageID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.Message{}, exceptions.ErrNotFound
	}

	if err != nil {
		return models.Message{}, fmt.Errorf("find message: %w", err)
	}

	if m.DeletedAt != nil || m.BodyFull == nil {
		return models.Message{}, exceptions.ErrNotFound
	}

	return m, nil
}
