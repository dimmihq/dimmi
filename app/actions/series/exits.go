/*
|--------------------------------------------------------------------------
| Leaving And Reaching The Goal
|--------------------------------------------------------------------------
|
| Before any step runs, the journey asks whether the traveler must
| leave: they match the exit rules, answered a series message when the
| series stops on replies, or unsubscribed when it stops on that. Then
| whether they reached the goal inside its window, counted from when
| they entered; a series can stop there too. Leaving is immediate:
| nothing queued for them goes out afterwards.
|
*/

package series

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
guard ends the journey when the traveler must leave, and records the
goal; left is true when the journey ended.
*/
func (r *run) guard(ctx context.Context) (bool, error) {
	reason, err := r.exitReason(ctx)
	if err != nil {
		return false, err
	}

	if reason != "" {
		return true, r.end(ctx, reason)
	}

	reached, err := r.reachedGoal(ctx)
	if err != nil || !reached {
		return false, err
	}

	if _, err := r.q.MarkSeriesGoal(ctx, models.MarkSeriesGoalParams{
		WorkspaceID: r.e.WorkspaceID, ID: r.e.ID, Now: r.now,
	}); err != nil {
		return false, wrap("mark goal", err)
	}

	r.e.GoalAt = &r.now

	if r.g.spec.Exit.OnGoal {
		return true, r.end(ctx, "goal")
	}

	return false, nil
}

/*
exitReason returns why the traveler must leave, or "" when they stay.
*/
func (r *run) exitReason(ctx context.Context) (string, error) {
	exit := r.g.spec.Exit
	if len(r.people) == 0 {
		return "subject_gone", nil
	}

	if len(exit.Rules) > 0 {
		match, err := r.matches(ctx, nil, exit.Rules)
		if err != nil {
			return "", err
		}

		if match {
			return "exit_rules", nil
		}
	}

	if !exit.OnReply && !exit.OnUnsubscribe {
		return "", nil
	}

	got, err := r.q.SeriesEnrollmentEngagement(ctx, models.SeriesEnrollmentEngagementParams{
		WorkspaceID: r.e.WorkspaceID, SeriesEnrollmentID: &r.e.ID,
	})
	if err != nil {
		return "", wrap("read engagement", err)
	}

	switch {
	case exit.OnReply && got.Replied:
		return "replied", nil
	case exit.OnUnsubscribe && got.Unsubscribed:
		return "unsubscribed", nil
	}

	return "", nil
}

/*
reachedGoal reports whether the traveler reached the goal inside its
window and had not before.
*/
func (r *run) reachedGoal(ctx context.Context) (bool, error) {
	goal := r.g.spec.Goal
	if goal == nil || r.e.GoalAt != nil || r.now.After(r.e.EnteredAt.Add(time.Duration(goal.Days)*day)) {
		return false, nil
	}

	if goal.Event != "" {
		return r.didEvent(ctx, goal.Event, r.e.EnteredAt)
	}

	if len(goal.Rules) == 0 {
		return false, nil
	}

	return r.matches(ctx, nil, goal.Rules)
}
