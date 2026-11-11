/*
|--------------------------------------------------------------------------
| Office Hours In Runs
|--------------------------------------------------------------------------
|
| Workflows read the team's hours from a schedule: the one a definition
| names, or else the office hours of the conversation's inbox, which are
| the workspace's default unless the inbox keeps its own. Whether the
| team is around is answered at the moment of the event or the step,
| in the schedule's own timezone. A schedule that cannot be read counts
| as closed, so a workflow restricted to open hours never runs on a
| guess.
|
*/

package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
SkipSchedule is the verdict of a workflow restricted to hours it is
not in.
*/
const SkipSchedule = "schedule"

/*
errScheduleMissing is a schedule a definition names that is gone.
*/
var errScheduleMissing = errors.New("schedule not found")

/*
hours returns the schedule the definition names by public id, or the
office hours of the conversation's inbox for none.
*/
func (e *Engine) hours(ctx context.Context, q *models.Queries, place schedules.Place, public string,
) (*officehours.Schedule, error) {
	workspace := place.Workspace
	if public == "" {
		row, err := schedules.ForInbox(ctx, q, place, e.Clock)
		if err != nil {
			return nil, err
		}

		return schedules.Read(row)
	}

	id, err := e.IDs.Decode(string(models.PrefixSchedule), public)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errScheduleMissing, public)
	}

	row, err := q.FindSchedule(ctx, models.FindScheduleParams{WorkspaceID: workspace, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s", errScheduleMissing, public)
	}

	if err != nil {
		return nil, fmt.Errorf("find schedule: %w", err)
	}

	return schedules.Read(row)
}

/*
scheduleProblem reports whether an error is about the schedule itself:
gone, unreadable, or never open. A step that needs it is skipped rather
than retried forever.
*/
func scheduleProblem(err error) bool {
	return errors.Is(err, errScheduleMissing) || errors.Is(err, officehours.ErrInvalid) ||
		errors.Is(err, officehours.ErrNeverOpen)
}

/*
withHours gives the subject its inbox's office hours, which the office
hours condition and the weekday read; with an unreadable schedule both
stay unknown.
*/
func (e *Engine) withHours(ctx context.Context, q *models.Queries, s *subject) error {
	sched, err := e.hours(ctx, q, placeOf(s.conversation), "")
	if scheduleProblem(err) {
		return nil
	}

	if err != nil {
		return err
	}

	s.hours, s.location = sched, sched.Location()

	return nil
}

/*
scheduleReason refuses a workflow restricted to open or closed hours
when the event happened outside them.
*/
func (e *Engine) scheduleReason(ctx context.Context, q *models.Queries, m *match, c candidate) (string, error) {
	mode := c.spec.Schedule.Mode
	if mode != "open" && mode != "closed" {
		return "", nil
	}

	sched, err := e.hours(ctx, q, placeOf(m.conv), c.spec.Schedule.ScheduleID)
	if scheduleProblem(err) {
		return SkipSchedule, nil
	}

	if err != nil {
		return "", err
	}

	if sched.IsOpen(e.Clock.Now()) != (mode == "open") {
		return SkipSchedule, nil
	}

	return "", nil
}

/*
nextOpen is when the schedule a wait or snooze names next opens: now
when it is open.
*/
func (st *state) nextOpen(ctx context.Context, u *workflowspec.WaitUntil) (time.Time, error) {
	sched, err := st.e.hours(ctx, st.q, placeOf(st.conv), u.ScheduleID)
	if err != nil {
		return time.Time{}, err
	}

	return sched.NextOpen(st.now())
}

/*
placeOf is the inbox whose office hours a conversation follows.
*/
func placeOf(c models.Conversation) schedules.Place {
	return schedules.Place{Workspace: c.WorkspaceID, Inbox: c.InboxID}
}
