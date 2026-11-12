/*
|--------------------------------------------------------------------------
| Import Message
|--------------------------------------------------------------------------
|
| History from another tool is brought in entry by entry, oldest first:
| a customer's message, a teammate's reply or an internal note, each at
| the time it was written and by whoever wrote it. The conversation's
| columns move as they would have then: the customer starts waiting at
| their first unanswered message, a reply ends the wait, and a customer
| writing to a closed conversation opens it again at that moment. A reply
| may close the conversation at its own time. Nothing is sent and
| nothing reacts. The id an entry had in the other tool makes sending it
| again find the first copy.
|
*/

package messages

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of entry history brings in.
*/
const (
	ImportContactMessage = "contact"
	ImportReply          = "reply"
	ImportNote           = "note"
)

/*
ImportMessageInput is who brings which entry into which conversation.
Kind is one of the Import kinds. Then, for a reply, is ThenKeep,
ThenOpen or ThenClose, the move made at the reply's time. MutationID is
the id the client gave the change, empty when it gave none.
*/
type ImportMessageInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	Kind       string
	Body       string
	Backfill   conversations.Backfill
	Then       string
	MutationID string
}

/*
ImportMessage brings one entry of history into a conversation.

Rules:
  - The caller needs conversations:write and imports:manage.
  - The time lies between the year 2000 and now, and not before the
    conversation's latest entry.
  - The same external id on the same conversation returns the entry
    already imported.
  - Any channel takes it; nothing is sent.

Events: none.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrImportTimeInFuture], [exceptions.ErrImportTimeTooEarly],
[exceptions.ErrImportOutOfOrder], or a wrapped database error.
*/
type ImportMessage struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle writes the entry and returns it.
*/
func (a *ImportMessage) Handle(ctx context.Context, in ImportMessageInput) (Written, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Written{}, err
	}

	if err := conversations.CanImport(in.Membership); err != nil {
		return Written{}, err
	}

	now := a.Clock.Now()
	at := in.Backfill.When(now)

	if err := conversations.CheckImportTime(at, now, "created_at"); err != nil {
		return Written{}, err
	}

	var out Written

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		services := conversations.Services{Events: a.Events, Clock: a.Clock}

		s, err := open(ctx, tx, services, target{in.Membership, in.Ref, in.MutationID})
		if err != nil {
			return err
		}

		s.w = s.w.Importing(at)
		if err := s.w.Q.MarkImportSource(ctx); err != nil {
			return fmt.Errorf("tag imported history: %w", err)
		}

		key := importKey(s.c, in.Backfill.ExternalID)
		if out, err = s.replay(ctx, key); err != nil || out.Replayed {
			return err
		}

		if at.Before(conversations.Latest(s.c)) {
			return exceptions.ErrImportOutOfOrder
		}

		if err := s.importEntry(ctx, in, key); err != nil {
			return err
		}

		out, err = s.written(ctx)

		return err
	})

	return out, err
}

/*
importKey returns the key an imported entry is kept under in the
conversation, nil without an external id.
*/
func importKey(c models.Conversation, externalID *string) *string {
	if externalID == nil {
		return nil
	}

	return new("import:" + c.ID.String() + ":" + *externalID)
}

/*
importEntry writes the entry as its author and makes the move it brings
about.
*/
func (s *session) importEntry(ctx context.Context, in ImportMessageInput, key *string) error {
	author, err := s.importAuthor(ctx, in)
	if err != nil {
		return err
	}

	kind := string(enums.MessageTypeMessage)
	if in.Kind == ImportNote {
		kind = string(enums.MessageTypeNote)
	}

	msg, err := conversations.ImportedMessage(conversations.ImportedEntry{
		Kind: kind, Body: in.Body, ByMember: in.Kind == ImportReply, Key: key,
	}, in.Backfill)
	if err != nil {
		return err
	}

	if s.msg, s.c, err = author.Append(ctx, s.c, msg); err != nil {
		return err
	}

	to := importedStatus(s.c, in)
	if to == "" {
		return nil
	}

	s.c, _, err = author.ChangeStatus(ctx, s.c, conversations.StatusChange{
		Status: to, Reason: importedReason(in.Kind),
	})

	return err
}

/*
importAuthor returns the writer acting as whoever wrote the entry: the
conversation's customer, or the member named, the caller when none is.
*/
func (s *session) importAuthor(ctx context.Context, in ImportMessageInput) (*conversations.Writer, error) {
	if in.Kind != ImportContactMessage {
		return s.w.AsAuthor(ctx, s.c.WorkspaceID, in.Backfill.AuthorID)
	}

	contact, err := s.w.Q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.c.WorkspaceID, ID: s.c.ContactID})
	if err != nil {
		return nil, fmt.Errorf("find contact: %w", err)
	}

	return s.w.AsContact(contact), nil
}

/*
importedStatus returns the status the entry moves the conversation to,
empty for none: a customer's message opens a conversation that is not
open or spam, and a reply opens or closes it as asked.
*/
func importedStatus(c models.Conversation, in ImportMessageInput) string {
	open, closed := string(enums.ConversationStatusOpen), string(enums.ConversationStatusClosed)

	switch {
	case in.Kind == ImportContactMessage && c.Status != open && !c.Spam:
		return open
	case in.Kind == ImportReply && in.Then == ThenClose && c.Status != closed:
		return closed
	case in.Kind == ImportReply && in.Then == ThenOpen && c.Status != open:
		return open
	}

	return ""
}

/*
importedReason returns why the timeline says the conversation moved: a
customer's message, or nothing for a teammate's own move.
*/
func importedReason(kind string) string {
	if kind == ImportContactMessage {
		return conversations.ReasonCustomerMessage
	}

	return ""
}
