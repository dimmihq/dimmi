/*
|--------------------------------------------------------------------------
| Read State
|--------------------------------------------------------------------------
|
| How far each teammate has read each conversation. A marker only moves
| forward when someone reads, so opening an old link never makes newer
| messages look read. Marking a conversation unread puts the marker just
| before the latest entry someone else wrote, so exactly that entry
| shows as new; a conversation with nothing from anyone else has nothing
| to mark. What a teammate writes, they have read.
|
| Read state is personal: the event about it concerns that member alone.
|
*/

package conversations

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReadState is how far the member has read the conversation: up to the
entry, when, and whether anything newer from someone else is waiting.
*/
type ReadState struct {
	ConversationID    ids.UUID
	LastReadMessageID *ids.UUID
	LastReadAt        *time.Time
	Unread            bool
}

/*
ReadStateInput is who asks about which conversation.
*/
type ReadStateInput struct {
	Membership authz.Membership
	Ref        Ref
}

/*
ShowReadState returns the caller's read state of a conversation.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowReadState struct {
	DB *database.DB
}

/*
Handle returns the read state.
*/
func (a *ShowReadState) Handle(ctx context.Context, in ReadStateInput) (ReadState, error) {
	if err := CanRead(in.Membership); err != nil {
		return ReadState{}, err
	}

	var out ReadState

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := Find(ctx, q, in.Membership, in.Ref)
		if err != nil {
			return err
		}

		out, err = LoadReadState(ctx, q, c, in.Membership.Member.ID)

		return err
	})

	return out, err
}

/*
UpdateReadStateInput is who moves their marker on which conversation:
up to the entry UpTo, to the latest entry when nil, or back to unread
when Unread. MutationID is the id the client gave the change, empty
when it gave none.
*/
type UpdateReadStateInput struct {
	Membership authz.Membership
	Ref        Ref
	UpTo       *ids.UUID
	Unread     bool
	MutationID string
}

/*
UpdateReadState moves the caller's read marker on a conversation.

Rules:
  - The caller needs conversations:read.
  - The entry read up to belongs to the conversation.
  - Reading moves forward only; an older entry changes nothing.

Events: [appevents.ConversationRead] when the marker moved.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMessageNotInConversation], or a wrapped database error.
*/
type UpdateReadState struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle moves the marker and returns the read state as it now is.
*/
func (a *UpdateReadState) Handle(ctx context.Context, in UpdateReadStateInput) (ReadState, error) {
	if err := CanRead(in.Membership); err != nil {
		return ReadState{}, err
	}

	var out ReadState

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w := &Writer{Q: models.New(tx), Tx: tx, Events: a.Events, Now: a.Clock.Now(), MutationID: in.MutationID}
		member := in.Membership.Member.ID

		c, err := Find(ctx, w.Q, in.Membership, in.Ref)
		if err != nil {
			return err
		}

		if in.Unread {
			_, err = w.MarkUnread(ctx, c, member)
		} else {
			err = w.markReadTo(ctx, c, member, in.UpTo)
		}

		if err != nil {
			return err
		}

		out, err = LoadReadState(ctx, w.Q, c, member)

		return err
	})

	return out, err
}

/*
markReadTo marks the conversation read up to the entry, refusing one of
another conversation.
*/
func (w *Writer) markReadTo(ctx context.Context, c models.Conversation, member ids.UUID, upTo *ids.UUID) error {
	if upTo == nil {
		_, err := w.MarkRead(ctx, c, member, nil)

		return err
	}

	entry, err := w.Q.FindMessage(ctx, models.FindMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ID: *upTo,
	})
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrMessageNotInConversation
	}

	if err != nil {
		return fmt.Errorf("find message: %w", err)
	}

	_, err = w.MarkRead(ctx, c, member, &entry)

	return err
}

/*
LoadReadState reads the member's read state of the conversation.
*/
func LoadReadState(ctx context.Context, q *models.Queries, c models.Conversation, member ids.UUID) (ReadState, error) {
	out := ReadState{ConversationID: c.ID}

	read, err := q.FindConversationRead(ctx, models.FindConversationReadParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, MemberID: member,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return out, fmt.Errorf("find read state: %w", err)
	}

	if err == nil {
		out.LastReadMessageID, out.LastReadAt = read.LastReadMessageID, &read.LastReadAt
	}

	unread, err := q.UnreadConversationIDs(ctx, models.UnreadConversationIDsParams{
		WorkspaceID: c.WorkspaceID, Ids: []ids.UUID{c.ID}, MemberID: member,
	})
	if err != nil {
		return out, fmt.Errorf("find unread state: %w", err)
	}

	out.Unread = len(unread) > 0

	return out, nil
}

/*
MarkRead moves the member's marker forward to the entry, or to the
latest entry when nil, and reports false when it was already there or
further on.
*/
func (w *Writer) MarkRead(
	ctx context.Context, c models.Conversation, member ids.UUID, upTo *models.Message,
) (bool, error) {
	if upTo == nil {
		latest, err := w.Q.LatestTimelineEntry(ctx, models.LatestTimelineEntryParams{
			WorkspaceID: c.WorkspaceID, ConversationID: c.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return false, nil
		}

		if err != nil {
			return false, fmt.Errorf("find latest entry: %w", err)
		}

		upTo = &latest
	}

	current, err := w.marker(ctx, c, member)
	if err != nil {
		return false, err
	}

	if current != nil && current.LastReadMessageID != nil &&
		bytes.Compare(current.LastReadMessageID[:], upTo.ID[:]) >= 0 {
		return false, nil
	}

	return true, w.saveMarker(ctx, c, member, mark{entry: &upTo.ID, at: upTo.CreatedAt})
}

/*
MarkUnread puts the member's marker just before the latest entry
someone else wrote, and reports false when there is none, the marker
already sits there, or the member never read anything.
*/
func (w *Writer) MarkUnread(ctx context.Context, c models.Conversation, member ids.UUID) (bool, error) {
	latest, err := w.Q.LatestForeignMessage(ctx, models.LatestForeignMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, MemberID: member,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find latest message: %w", err)
	}

	var target *ids.UUID

	at := latest.CreatedAt

	before, err := w.Q.TimelineEntryBefore(ctx, models.TimelineEntryBeforeParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, BeforeID: latest.ID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return false, fmt.Errorf("find entry before: %w", err)
	}

	if err == nil {
		target, at = &before.ID, before.CreatedAt
	}

	current, err := w.marker(ctx, c, member)
	if err != nil {
		return false, err
	}

	if (current == nil && target == nil) || (current != nil && sameID(current.LastReadMessageID, target)) {
		return false, nil
	}

	return true, w.saveMarker(ctx, c, member, mark{entry: target, at: at, unread: true})
}

/*
marker returns the member's marker, nil when they never read the
conversation.
*/
func (w *Writer) marker(
	ctx context.Context, c models.Conversation, member ids.UUID,
) (*models.ConversationRead, error) {
	read, err := w.Q.FindConversationRead(ctx, models.FindConversationReadParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, MemberID: member,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find read state: %w", err)
	}

	return &read, nil
}

/*
mark is where a marker goes: the entry and its time, and whether the
move marks the conversation unread.
*/
type mark struct {
	entry  *ids.UUID
	at     time.Time
	unread bool
}

/*
saveMarker writes the member's marker and says so.
*/
func (w *Writer) saveMarker(ctx context.Context, c models.Conversation, member ids.UUID, m mark) error {
	_, err := w.Q.SaveConversationRead(ctx, models.SaveConversationReadParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, MemberID: member, LastReadMessageID: m.entry,
		LastReadAt: m.at, Now: w.Now,
	})
	if err != nil {
		return fmt.Errorf("save read state: %w", err)
	}

	return w.Dispatch(ctx, appevents.ConversationRead{
		Base: w.Base(c.WorkspaceID), ConversationRef: ref(c), MemberRef: appevents.MemberRef{MemberID: member},
		LastReadMessageID: m.entry, Unread: m.unread,
	})
}
