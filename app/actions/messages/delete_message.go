/*
|--------------------------------------------------------------------------
| Delete Message
|--------------------------------------------------------------------------
|
| A teammate removes what should not have been written: their own reply
| or note at any time, a colleague's or the customer's only when they
| may moderate, such as for a pasted card number. A reply still waiting
| to leave is undone instead. A deleted message keeps its place as a
| tombstone saying who removed it; its text, files and every copy other
| parts keep are erased. A copy the customer already received stays
| theirs.
|
*/

package messages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Removed is a message taken away: undone when it was a reply still
waiting, with its text for the composer, and the conversation as it now
is.
*/
type Removed struct {
	Undone       bool
	Body         string
	Conversation conversations.Loaded
}

/*
DeleteMessage deletes a message, or undoes a reply still waiting.

Rules:
  - The caller needs conversations:write.
  - Their own reply still waiting to leave is undone.
  - Their own reply or note goes at any time; anyone else's message,
    the customer's included, needs conversations:moderate.
  - Timeline events cannot be deleted.

Events: [appevents.MessageDeleted], and those of undoing.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMessageNotDeletable], or a wrapped database error.
*/
type DeleteMessage struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Queue  *queue.Queue
}

/*
Handle removes the message.
*/
func (a *DeleteMessage) Handle(ctx context.Context, in UndoInput) (Removed, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Removed{}, err
	}

	var out Removed

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s, err := open(ctx, tx, conversations.Services{Events: a.Events, Clock: a.Clock}, target{
			in.Membership, in.Ref, in.MutationID,
		})
		if err != nil {
			return err
		}

		if out, err = a.remove(ctx, s, in.MessageID); err != nil {
			return err
		}

		out.Conversation, err = conversations.LoadOne(ctx, s.w.Q, in.Membership.Member.ID, s.c)

		return err
	})

	return out, err
}

/*
remove undoes the caller's waiting reply, or deletes the message.
*/
func (a *DeleteMessage) remove(ctx context.Context, s *session, id ids.UUID) (Removed, error) {
	msg, err := s.w.Q.LockMessage(ctx, models.LockMessageParams{
		WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && (msg.DeletedAt != nil || msg.UndoneAt != nil)) {
		return Removed{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Removed{}, fmt.Errorf("find message: %w", err)
	}

	if s.wrote(msg) && msg.ScheduledSendAt != nil && msg.ScheduledSendAt.After(s.w.Now) {
		body, err := s.undo(ctx, msg)

		return Removed{Undone: true, Body: body}, err
	}

	if err := s.mayDelete(msg); err != nil {
		return Removed{}, err
	}

	return Removed{}, a.erase(ctx, s, msg)
}

/*
mayDelete refuses events, and anyone else's message to a caller who
may not moderate.
*/
func (s *session) mayDelete(msg models.Message) error {
	if msg.Type == string(enums.MessageTypeEvent) {
		return exceptions.ErrMessageNotDeletable
	}

	own := msg.AuthorType == string(enums.AuthorTypeMember) && msg.AuthorID != nil && *msg.AuthorID == s.m.Member.ID
	if !own && !s.m.Can("conversations:moderate") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
erase empties the message, takes it out of the conversation's counts,
announces it and queues the erasing of its files and copies.
*/
func (a *DeleteMessage) erase(ctx context.Context, s *session, msg models.Message) error {
	by := map[string]string{"id": models.PublicID(models.PrefixMember, s.w.Actor.ID), "name": s.w.Actor.Name}

	raw, err := json.Marshal(map[string]any{"deleted_by": by})
	if err != nil {
		return fmt.Errorf("encode deletion: %w", err)
	}

	if s.msg, err = s.w.Q.DeleteMessage(ctx, models.DeleteMessageParams{
		Now: s.w.Now, Metadata: raw, WorkspaceID: s.c.WorkspaceID, ID: msg.ID,
	}); err != nil {
		return fmt.Errorf("delete message: %w", err)
	}

	if msg.Type == string(enums.MessageTypeMessage) {
		if s.c, err = s.w.Q.UncountDeletedMessage(ctx, models.UncountDeletedMessageParams{
			Now: s.w.Now, WorkspaceID: s.c.WorkspaceID, ID: s.c.ID,
		}); err != nil {
			return fmt.Errorf("uncount message: %w", err)
		}
	}

	r := appevents.MessageRef{ConversationRef: appevents.ConversationRef{ConversationID: s.c.ID}, MessageID: msg.ID}
	if err := s.w.Dispatch(ctx, appevents.MessageDeleted{Base: s.w.Base(s.c.WorkspaceID), MessageRef: r}); err != nil {
		return err
	}

	err = a.Queue.PushTx(ctx, s.w.Tx, jobs.EraseMessageCopiesArgs{WorkspaceID: s.c.WorkspaceID, MessageID: msg.ID})
	if err != nil {
		return fmt.Errorf("queue erasing: %w", err)
	}

	return nil
}
