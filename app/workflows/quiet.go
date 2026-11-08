/*
|--------------------------------------------------------------------------
| Quiet Triggers
|--------------------------------------------------------------------------
|
| "Customer stops replying" starts from a public reply by a teammate or
| dimmi and fires when the customer said nothing for the workflow's
| wait; a newer reply starts a new silence, so each reply fires at most
| once. "Customer waits for the team" starts when the customer begins
| waiting and fires when nobody on the team replied for the wait; a
| reply ends the waiting period, and the next one fires again. Either
| fires only on an open conversation. The wait may count only the
| hours a schedule is open.
|
*/

package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
ArmQuiet arms one timer for each live workflow listening for the
silence the message may start.
*/
func (e *Engine) ArmQuiet(ctx context.Context, a jobs.ArmQuietTriggersArgs) error {
	return e.transaction(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		found, err := e.candidates(ctx, q, a.WorkspaceID, a.Trigger)
		if err != nil || len(found) == 0 {
			return err
		}

		conv, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: a.WorkspaceID, ID: a.ConversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find conversation: %w", err)
		}

		anchor, ok := quietAnchor(a, conv, e.Clock.Now())
		if !ok {
			return nil
		}

		for _, c := range found {
			if err := e.armOne(ctx, tx, quietArm{args: a, workflow: c, anchor: anchor}); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
quietAnchor is when the silence starts: now, for a reply, or when the
customer began waiting.
*/
func quietAnchor(a jobs.ArmQuietTriggersArgs, conv models.Conversation, now time.Time) (time.Time, bool) {
	if a.Trigger != workflowspec.TriggerTeammateQuiet {
		return now, true
	}

	if conv.WaitingSince == nil {
		return time.Time{}, false
	}

	return *conv.WaitingSince, true
}

/*
quietArm is one timer to arm: the message that may start the silence,
the workflow listening for it, and when the silence starts.
*/
type quietArm struct {
	args     jobs.ArmQuietTriggersArgs
	workflow candidate
	anchor   time.Time
}

/*
armOne arms one workflow's timer, when the workflow listens for the
reply's author.
*/
func (e *Engine) armOne(ctx context.Context, tx *database.Tx, arm quietArm) error {
	a, c, anchor := arm.args, arm.workflow, arm.anchor
	settings := workflowspec.ReadQuiet(c.spec.TriggerSettings)
	if a.Trigger == workflowspec.TriggerCustomerQuiet && !repliesFrom(settings.AfterReplyFrom, a.Author) {
		return nil
	}

	due, err := e.quietDue(ctx, models.New(tx), a.WorkspaceID, quietWait{settings: settings, anchor: anchor})
	if scheduleProblem(err) {
		return nil
	}

	if err != nil {
		return err
	}

	args := jobs.FireQuietTriggerArgs{
		WorkspaceID: a.WorkspaceID, ConversationID: a.ConversationID, WorkflowID: c.workflowID,
		Trigger: a.Trigger, AnchorID: a.MessageID, AnchorAt: anchor, Due: due,
	}
	if a.Trigger == workflowspec.TriggerTeammateQuiet {
		args.AnchorID = ids.Nil()
	}

	if err := e.Queue.PushTx(ctx, tx, args, queue.At(due)); err != nil {
		return fmt.Errorf("arm quiet trigger: %w", err)
	}

	return nil
}

/*
repliesFrom reports whether a reply's author starts the silence a
workflow waits out.
*/
func repliesFrom(setting, author string) bool {
	switch setting {
	case "member":
		return author == string(enums.AuthorTypeMember)
	case "dimmi":
		return author == string(enums.AuthorTypeAgent)
	}

	return true
}

/*
quietWait is a silence to wait out: the trigger's settings and when it
started.
*/
type quietWait struct {
	settings workflowspec.QuietSettings
	anchor   time.Time
}

/*
quietDue is when a silence started at the anchor has lasted the
workflow's wait, counting only open hours when it counts business
time.
*/
func (e *Engine) quietDue(ctx context.Context, q *models.Queries, workspace ids.UUID, w quietWait,
) (time.Time, error) {
	s, anchor := w.settings, w.anchor
	d, err := workflowspec.ParseDuration(s.After)
	if err != nil {
		return time.Time{}, fmt.Errorf("read quiet wait: %w", err)
	}

	if s.Count != "business" {
		return anchor.Add(d), nil
	}

	sched, err := e.hours(ctx, q, workspace, s.ScheduleID)
	if err != nil {
		return time.Time{}, err
	}

	return sched.AddBusiness(anchor, d)
}

/*
FireQuiet offers the quiet trigger to the timer's workflow when the
silence still holds. A timer that comes early waits until it is due.
*/
func (e *Engine) FireQuiet(ctx context.Context, a jobs.FireQuietTriggerArgs) error {
	if early := a.Due.Sub(e.Clock.Now()); early > 0 {
		return queue.Postpone(early)
	}

	holds, err := e.silenceHolds(ctx, a)
	if err != nil || !holds {
		return err
	}

	return e.Match(ctx, Event{
		WorkspaceID: a.WorkspaceID, ConversationID: a.ConversationID, Trigger: a.Trigger, Only: &a.WorkflowID,
		EventID: "quiet:" + a.WorkflowID.String() + ":" + a.AnchorAt.UTC().Format(time.RFC3339Nano),
		Data:    map[string]any{"trigger.quiet_since": a.AnchorAt},
	})
}

/*
silenceHolds reports whether the conversation is still open and still
silent the way the timer expects.
*/
func (e *Engine) silenceHolds(ctx context.Context, a jobs.FireQuietTriggerArgs) (bool, error) {
	holds := false
	err := e.transaction(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		conv, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: a.WorkspaceID, ID: a.ConversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find conversation: %w", err)
		}

		if conv.Status != string(enums.ConversationStatusOpen) || !matchable(conv) {
			return nil
		}

		if a.Trigger == workflowspec.TriggerTeammateQuiet {
			holds = conv.WaitingSince != nil && conv.WaitingSince.Equal(a.AnchorAt)
			return nil
		}

		holds, err = customerSilent(ctx, q, a)

		return err
	})

	return holds, err
}

/*
customerSilent reports whether the anchor reply is still the newest
one and the customer wrote nothing since.
*/
func customerSilent(ctx context.Context, q *models.Queries, a jobs.FireQuietTriggerArgs) (bool, error) {
	anchors, err := q.QuietAnchors(ctx, models.QuietAnchorsParams{
		WorkspaceID: a.WorkspaceID, ConversationID: a.ConversationID, AnchorID: a.AnchorID,
	})
	if err != nil {
		return false, fmt.Errorf("check silence: %w", err)
	}

	return anchors.LastReplyID == a.AnchorID && !anchors.CustomerWrote, nil
}
