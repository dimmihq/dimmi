/*
|--------------------------------------------------------------------------
| Undo Restore
|--------------------------------------------------------------------------
|
| Undoing a reply puts the conversation back as it was before the reply,
| one field at a time, and only where the field still holds what the
| reply left there: a teammate who reassigned or reopened it meanwhile
| keeps their change.
|
*/

package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
restore takes the undone reply out of the conversation's counts and
puts back each field it changed that nobody changed since.
*/
func (s *session) restore(ctx context.Context, msg models.Message) error {
	var record struct {
		Undo held `json:"undo"`
	}

	if err := json.Unmarshal(msg.Metadata, &record); err != nil {
		return fmt.Errorf("read undo: %w", err)
	}

	was, now := record.Undo, stateOf(s.c)

	if _, err := s.w.Q.UncountConversationMessage(ctx, models.UncountConversationMessageParams{
		WaitingSince: back(now.WaitingSince, was.After.WaitingSince, was.Before.WaitingSince),
		FirstReplyAt: back(now.FirstReplyAt, was.After.FirstReplyAt, was.Before.FirstReplyAt),
		LastReplyAt:  back(now.LastReplyAt, was.After.LastReplyAt, was.Before.LastReplyAt),
		Now:          s.w.Now, WorkspaceID: s.c.WorkspaceID, ID: s.c.ID,
	}); err != nil {
		return fmt.Errorf("uncount reply: %w", err)
	}

	status, until := now.Status, now.SnoozedUntil
	if now.Status == was.After.Status && sameTime(now.SnoozedUntil, was.After.SnoozedUntil) {
		status, until = was.Before.Status, was.Before.SnoozedUntil
	}

	assignee := now.AssigneeMemberID
	if sameID(now.AssigneeMemberID, was.After.AssigneeMemberID) {
		assignee = was.Before.AssigneeMemberID
	}

	c, err := s.w.Q.RestoreConversationState(ctx, models.RestoreConversationStateParams{
		Status: status, SnoozedUntil: until, AssigneeMemberID: assignee, Now: s.w.Now,
		WorkspaceID: s.c.WorkspaceID, ID: s.c.ID,
	})
	if err != nil {
		return fmt.Errorf("restore conversation: %w", err)
	}

	changes := changed(s.c, c)
	s.c = c

	if len(changes) == 0 {
		return nil
	}

	return s.w.Dispatch(ctx, appevents.ConversationUpdated{
		Base: s.w.Base(c.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: c.ID},
		Changes: changes,
	})
}

/*
changed lists the fields undoing moved, each with its old and new value.
*/
func changed(from, to models.Conversation) map[string][2]any {
	out := map[string][2]any{}

	if from.Status != to.Status {
		out["status"] = [2]any{from.Status, to.Status}
	}

	if !sameID(from.AssigneeMemberID, to.AssigneeMemberID) {
		out["assignee_member_id"] = [2]any{from.AssigneeMemberID, to.AssigneeMemberID}
	}

	if !sameTime(from.SnoozedUntil, to.SnoozedUntil) {
		out["snoozed_until"] = [2]any{from.SnoozedUntil, to.SnoozedUntil}
	}

	return out
}

/*
back returns the value before the reply when the field still holds what
the reply left, and the current value otherwise.
*/
func back(current, after, before *time.Time) *time.Time {
	if sameTime(current, after) {
		return before
	}

	return current
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Equal(*b)
}

func sameID(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
