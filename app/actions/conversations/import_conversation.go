/*
|--------------------------------------------------------------------------
| Import Conversation
|--------------------------------------------------------------------------
|
| A conversation from another tool starts the way it started there: at
| its own time, with its first message by the customer or by the
| teammate who wrote it, on any of the inbox's channels, and closed when
| it was closed. Its source says it was imported and keeps the id it had
| there, so sending it again finds the first copy. Nobody is handed it
| unless the request names someone, and nothing reacts to it.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationImport is how a conversation is brought in: when and by
whom its first message was written, the id it had in the other tool,
and whether it is closed, with when, its first message's time when
ClosedAt is nil.
*/
type ConversationImport struct {
	Backfill
	Closed   bool
	ClosedAt *time.Time
}

/*
Import brings a conversation in from another tool with its first
message, and reports whether one with the same external id was already
there, in which case that one is returned untouched.

Rules:
  - The caller needs conversations:write and imports:manage.
  - Times lie between the year 2000 and now; a close is not before the
    first message.
  - Any channel of the inbox takes it; nothing is sent on it.

Events: none.

Errors: those of Handle, [exceptions.ErrImportTimeInFuture],
[exceptions.ErrImportTimeTooEarly] and
[exceptions.ErrImportClosedBeforeStart].
*/
func (a *CreateConversation) Import(
	ctx context.Context, in CreateConversationInput, imp ConversationImport,
) (Loaded, bool, error) {
	if err := CanWrite(in.Membership); err != nil {
		return Loaded{}, false, err
	}

	if err := CanImport(in.Membership); err != nil {
		return Loaded{}, false, err
	}

	now := a.Clock.Now()
	at := imp.When(now)

	closedAt, err := checkConversationTimes(imp, at, now)
	if err != nil {
		return Loaded{}, false, err
	}

	var (
		out      Loaded
		replayed bool
	)

	run := importRun{in: in, imp: imp, now: now, at: at, closedAt: closedAt}

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, replayed, err = a.importIn(ctx, tx, run)

		return err
	})

	return out, replayed, err
}

/*
importRun is one conversation being imported: the request, how it is
brought in, the server's time, when the conversation started and when
it closed.
*/
type importRun struct {
	in                CreateConversationInput
	imp               ConversationImport
	now, at, closedAt time.Time
}

/*
importIn imports the conversation inside the transaction, and reports
whether one with the same external id was already there, which is left
untouched.
*/
func (a *CreateConversation) importIn(ctx context.Context, tx *database.Tx, run importRun) (Loaded, bool, error) {
	live, err := NewWriter(ctx, tx, Services{Events: a.Events, Clock: a.Clock}, run.in.Membership)
	if err != nil {
		return Loaded{}, false, err
	}

	w := live.Importing(run.at)
	w.MutationID = run.in.MutationID

	if err := w.Q.MarkImportSource(ctx); err != nil {
		return Loaded{}, false, fmt.Errorf("tag imported history: %w", err)
	}

	c, found, err := findImported(ctx, w.Q, run.in.Membership.Workspace.ID, run.imp.ExternalID)
	if err != nil {
		return Loaded{}, false, err
	}

	if found && !VisibleScope(run.in.Membership).Contains(c) {
		return Loaded{}, false, exceptions.ErrForbidden
	}

	if !found {
		if c, err = a.replay(ctx, w, run); err != nil {
			return Loaded{}, false, err
		}

		if c, err = closeImported(ctx, w, c, run); err != nil {
			return Loaded{}, false, err
		}
	}

	out, err := LoadOne(ctx, w.Q, run.in.Membership.Member.ID, c)

	return out, found, err
}

/*
checkConversationTimes refuses times outside history and a close before
the start, and returns when the conversation closed.
*/
func checkConversationTimes(imp ConversationImport, at, now time.Time) (time.Time, error) {
	if err := CheckImportTime(at, now, "created_at"); err != nil {
		return at, err
	}

	if !imp.Closed || imp.ClosedAt == nil {
		return at, nil
	}

	if err := CheckImportTime(*imp.ClosedAt, now, "closed_at"); err != nil {
		return at, err
	}

	if imp.ClosedAt.Before(at) {
		return at, exceptions.ErrImportClosedBeforeStart
	}

	return *imp.ClosedAt, nil
}

/*
findImported returns the conversation imported under the external id,
and reports whether there is one.
*/
func findImported(
	ctx context.Context, q *models.Queries, ws ids.UUID, externalID *string,
) (models.Conversation, bool, error) {
	if externalID == nil {
		return models.Conversation{}, false, nil
	}

	c, err := q.FindConversationByImportExternalID(ctx, models.FindConversationByImportExternalIDParams{
		WorkspaceID: ws, ExternalID: *externalID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return c, false, nil
	}

	if err != nil {
		return c, false, fmt.Errorf("find imported conversation: %w", err)
	}

	return c, true, nil
}

/*
replay writes the conversation, its first message and its assignee at
the time it started.
*/
func (a *CreateConversation) replay(ctx context.Context, w *Writer, run importRun) (models.Conversation, error) {
	in, imp := run.in, run.imp

	o, err := a.importPlan(ctx, w, in, run.now)
	if err != nil {
		return models.Conversation{}, err
	}

	if o.source, err = importSource(imp.ExternalID); err != nil {
		return models.Conversation{}, err
	}

	c, err := o.insert(ctx, w)
	if err != nil {
		return c, err
	}

	author := w.AsContact(o.contact)
	if !in.ByContact {
		if author, err = w.AsAuthor(ctx, c.WorkspaceID, imp.AuthorID); err != nil {
			return c, err
		}
	}

	msg, err := ImportedMessage(ImportedEntry{
		Kind: string(enums.MessageTypeMessage), Body: in.Body, ByMember: !in.ByContact,
	}, imp.Backfill)
	if err != nil {
		return c, err
	}

	if _, c, err = author.Append(ctx, c, msg); err != nil {
		return c, err
	}

	if !in.AssignSet || in.Assignee == nil {
		return c, nil
	}

	c, _, err = w.Assign(ctx, c, Assignment{Member: in.Assignee})

	return c, err
}

/*
importPlan finds the channel and the customer. Unlike a live start, any
channel of the inbox takes the conversation, since nothing is sent on
it.
*/
func (a *CreateConversation) importPlan(
	ctx context.Context, w *Writer, in CreateConversationInput, now time.Time,
) (opening, error) {
	o := opening{in: in}
	ws := in.Membership.Workspace.ID

	lookup := in
	lookup.ByContact = false

	var err error
	if o.conn, err = a.connection(ctx, w.Q, ws, lookup); err != nil {
		return o, err
	}

	if !VisibleScope(in.Membership).OpensInbox(o.conn.InboxID) {
		return o, exceptions.ErrInboxNotFound
	}

	ref := in.Contact
	ref.WorkspaceID = ws
	o.contact, err = contacts.Resolve(ctx, w.Tx, now, ref)

	return o, err
}

/*
importSource returns the source of an imported conversation.
*/
func importSource(externalID *string) (json.RawMessage, error) {
	return ImportMetadata(externalID)
}

/*
closeImported closes a newly imported conversation at the time it
closed, as the writer's actor, when the import says it is closed.
*/
func closeImported(
	ctx context.Context, w *Writer, c models.Conversation, run importRun,
) (models.Conversation, error) {
	imp, closedAt := run.imp, run.closedAt
	if !imp.Closed {
		return c, nil
	}

	if imp.ClosedAt == nil && c.LastMessageAt != nil {
		closedAt = *c.LastMessageAt
	}

	closer, err := w.Importing(closedAt).AsAuthor(ctx, c.WorkspaceID, imp.AuthorID)
	if err != nil {
		return c, err
	}

	c, _, err = closer.ChangeStatus(ctx, c, StatusChange{Status: string(enums.ConversationStatusClosed)})

	return c, err
}
