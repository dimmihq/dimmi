/*
|--------------------------------------------------------------------------
| Undo Send
|--------------------------------------------------------------------------
|
| A reply on a channel that cannot take a message back, such as email,
| waits a few seconds before it leaves when its writer asked for that
| from the dashboard. Within the wait the writer can undo it: the reply
| is emptied and shown nowhere, and what it changed on the conversation
| goes back, field by field, unless someone changed that field since.
| Once the wait is over the reply has left and undo is refused.
|
*/

package messages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
held is what a waiting reply changed on its conversation: the fields as
they were before it and as it left them.
*/
type held struct {
	Before heldState `json:"before"`
	After  heldState `json:"after"`
}

/*
heldState is the part of a conversation undoing a reply puts back.
*/
type heldState struct {
	WaitingSince     *time.Time `json:"waiting_since"`
	FirstReplyAt     *time.Time `json:"first_reply_at"`
	LastReplyAt      *time.Time `json:"last_reply_at"`
	Status           string     `json:"status"`
	SnoozedUntil     *time.Time `json:"snoozed_until"`
	AssigneeMemberID *ids.UUID  `json:"assignee_member_id"`
}

func stateOf(c models.Conversation) heldState {
	return heldState{
		WaitingSince: c.WaitingSince, FirstReplyAt: c.FirstReplyAt, LastReplyAt: c.LastReplyAt, Status: c.Status,
		SnoozedUntil: c.SnoozedUntil, AssigneeMemberID: c.AssigneeMemberID,
	}
}

/*
undoWindow returns how long the caller's replies on the route wait when
they ask to: none when the channel shows a message the moment it is
written; otherwise the member's own choice, else the workspace's.
*/
func undoWindow(m authz.Membership, route conversations.Route) time.Duration {
	if route.Status != string(enums.DeliveryStatusPending) {
		return 0
	}

	seconds := m.Workspace.UndoSendSeconds()
	if m.Member.UndoSendSeconds != nil {
		seconds = int(*m.Member.UndoSendSeconds)
	}

	return time.Duration(seconds) * time.Second
}

/*
hold makes the reply wait until the window ends, remembering the
conversation as it was before the reply.
*/
func (s *session) hold(ctx context.Context, window time.Duration, before heldState) error {
	raw, err := json.Marshal(map[string]held{"undo": {Before: before}})
	if err != nil {
		return fmt.Errorf("encode undo: %w", err)
	}

	s.msg, err = s.w.Q.ScheduleMessage(ctx, models.ScheduleMessageParams{
		ScheduledSendAt: new(s.w.Now.Add(window)), Metadata: raw, Now: s.w.Now,
		WorkspaceID: s.c.WorkspaceID, ID: s.msg.ID,
	})
	if err != nil {
		return fmt.Errorf("hold reply: %w", err)
	}

	return nil
}

/*
settle records how the reply left the conversation, once it is done
with it, so undoing puts back only what is still the reply's doing.
*/
func (s *session) settle(ctx context.Context, before heldState) error {
	raw, err := json.Marshal(map[string]held{"undo": {Before: before, After: stateOf(s.c)}})
	if err != nil {
		return fmt.Errorf("encode undo: %w", err)
	}

	s.msg, err = s.w.Q.ScheduleMessage(ctx, models.ScheduleMessageParams{
		ScheduledSendAt: s.msg.ScheduledSendAt, Metadata: raw, Now: s.w.Now,
		WorkspaceID: s.c.WorkspaceID, ID: s.msg.ID,
	})
	if err != nil {
		return fmt.Errorf("hold reply: %w", err)
	}

	return nil
}

/*
UndoInput is who undoes which of their replies.
*/
type UndoInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	MessageID  ids.UUID
	MutationID string
}

/*
Undone is a reply taken back: its text, for the composer to hold again,
and the conversation as it now is.
*/
type Undone struct {
	Body         string
	Conversation conversations.Loaded
}

/*
UndoReply takes back a reply still waiting to leave.

Rules:
  - The caller needs conversations:write.
  - Only the reply's writer may undo it; anyone else finds no reply.
  - A reply no longer waiting has left, and is refused.
  - Each field the reply changed goes back unless it changed since.

Events: [appevents.MessageDeliveryUpdated] failed for good,
[appevents.MessageDeleted] and [appevents.ConversationUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrAlreadySent], or a wrapped database error.
*/
type UndoReply struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle undoes the reply.
*/
func (a *UndoReply) Handle(ctx context.Context, in UndoInput) (Undone, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Undone{}, err
	}

	var out Undone

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s, err := open(ctx, tx, conversations.Services{Events: a.Events, Clock: a.Clock}, target{
			in.Membership, in.Ref, in.MutationID,
		})
		if err != nil {
			return err
		}

		if out.Body, err = s.undo(ctx, in.MessageID); err != nil {
			return err
		}

		out.Conversation, err = conversations.LoadOne(ctx, s.w.Q, in.Membership.Member.ID, s.c)

		return err
	})

	return out, err
}

/*
undo takes the caller's waiting reply back and returns its text.
*/
func (s *session) undo(ctx context.Context, id ids.UUID) (string, error) {
	msg, err := s.w.Q.LockMessage(ctx, models.LockMessageParams{
		WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && !s.wrote(msg)) {
		return "", exceptions.ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf("find reply: %w", err)
	}

	if msg.ScheduledSendAt == nil || !msg.ScheduledSendAt.After(s.w.Now) {
		return "", exceptions.ErrAlreadySent
	}

	if s.msg, err = s.w.Q.UndoMessage(ctx, models.UndoMessageParams{
		Now: s.w.Now, WorkspaceID: s.c.WorkspaceID, ID: msg.ID,
	}); err != nil {
		return "", fmt.Errorf("undo reply: %w", err)
	}

	if err := s.restore(ctx, msg); err != nil {
		return "", err
	}

	return *msg.Body, s.announce(ctx)
}

/*
wrote reports whether the caller wrote the live reply.
*/
func (s *session) wrote(msg models.Message) bool {
	return msg.UndoneAt == nil && msg.DeletedAt == nil && msg.Body != nil &&
		msg.Type == string(enums.MessageTypeMessage) && msg.AuthorType == string(enums.AuthorTypeMember) &&
		msg.AuthorID != nil && *msg.AuthorID == s.m.Member.ID
}

/*
announce tells everyone the reply failed for good, canceled, and is
gone.
*/
func (s *session) announce(ctx context.Context) error {
	r := appevents.MessageRef{ConversationRef: appevents.ConversationRef{ConversationID: s.c.ID}, MessageID: s.msg.ID}

	if err := s.w.Dispatch(ctx, appevents.MessageDeliveryUpdated{
		Base: s.w.Base(s.c.WorkspaceID), MessageRef: r,
		Status: string(enums.DeliveryStatusFailed), Final: true, Reason: "canceled",
	}); err != nil {
		return err
	}

	return s.w.Dispatch(ctx, appevents.MessageDeleted{Base: s.w.Base(s.c.WorkspaceID), MessageRef: r})
}
