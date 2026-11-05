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
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
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
UndoInput is who removes which message: undoing their reply still
waiting, or deleting it.
*/
type UndoInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	MessageID  ids.UUID
	MutationID string
}

/*
undo takes back the caller's reply, locked and still waiting, and
returns its text.
*/
func (s *session) undo(ctx context.Context, msg models.Message) (string, error) {
	var err error

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
