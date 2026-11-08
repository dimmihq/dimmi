/*
|--------------------------------------------------------------------------
| Branches and Waits
|--------------------------------------------------------------------------
|
| A branch checks its paths top to bottom and takes the first whose
| condition holds, or "everyone else". The trace keeps the first
| condition that failed on each path it passed, so a wrong turn can be
| explained. A wait pauses the run for a while or until a time of day,
| and ends early when the customer or a teammate writes, if the step
| says so.
|
*/

package workflows

import (
	"context"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/app/support/audience"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
hour is an hour, the window of hourly limits.
*/
const hour = time.Hour

/*
The outcomes of logic steps.
*/
const (
	outcomeMatched = "matched"
	outcomeElapsed = "elapsed"
)

/*
runBranch takes the first path whose condition holds.
*/
func runBranch(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	subj, err := st.subject(ctx)
	if err != nil {
		return outcome{}, err
	}

	failed := map[string]string{}

	for _, p := range buttonPaths(s) {
		rule, err := audience.ParseRule(p.Condition)
		if err != nil {
			continue
		}

		program, err := audience.CompileRule(rule, workflowspec.Catalog(), audience.WorkflowBranch())
		if err != nil {
			continue
		}

		if err := subj.withSaved(ctx, st.q, program); err != nil {
			return outcome{}, err
		}

		explained := program.Explain(subj.env())
		if explained.Result == audience.True {
			return outcome{path: p.ID, outcome: outcomeMatched, detail: map[string]any{"failed": failed}}, nil
		}

		if leaf, ok := explained.FirstFailing(); ok {
			failed[p.ID] = leaf.Field
		}
	}

	out := fixedOrNext(s, workflowspec.OutcomeElse)
	out.detail = map[string]any{"failed": failed}

	return out, nil
}

/*
runWait pauses the run.
*/
func runWait(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	w := settings[workflowspec.WaitSettings](s)

	until, err := st.waitEnd(ctx, w)
	if scheduleProblem(err) {
		return skipped(outcomeMissingRef), nil
	}

	if err != nil {
		return outcome{}, err
	}

	interrupt := w.InterruptOn
	if len(interrupt) == 0 {
		interrupt = []string{interruptCustomer}
	}

	return outcome{wait: &waitState{Kind: waitTimer, Until: &until, Interrupt: interrupt}}, nil
}

/*
waitEnd is when a wait ends: after its duration, counting only open
hours when it counts business time, at the next opening of its
schedule, or at the next time of day.
*/
func (st *state) waitEnd(ctx context.Context, w workflowspec.WaitSettings) (time.Time, error) {
	now := st.now()
	switch {
	case w.Until != nil && w.Until.Kind == untilOpen:
		return st.nextOpen(ctx, w.Until)
	case w.Until != nil:
		return st.nextTimeOfDay(ctx, w.Until.Time), nil
	}

	d, _ := workflowspec.ParseDuration(w.For)
	if w.Count != "business" || d == 0 {
		return now.Add(d), nil
	}

	sched, err := st.e.hours(ctx, st.q, st.conv.WorkspaceID, w.ScheduleID)
	if err != nil {
		return time.Time{}, err
	}

	return sched.AddBusiness(now, d)
}

/*
untilOpen is the moment kind of waiting for office hours to open.
*/
const untilOpen = "office_hours_open"

/*
nextTimeOfDay is the next moment the clock shows the time in the
workspace's time zone.
*/
func (st *state) nextTimeOfDay(ctx context.Context, clock string) time.Time {
	at, err := time.Parse("15:04", clock)
	now := st.now()

	if err != nil {
		return now
	}

	loc := st.location(ctx)
	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), at.Hour(), at.Minute(), 0, 0, loc)

	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}

	return next
}

/*
location is the workspace's time zone, UTC when it has none.
*/
func (st *state) location(ctx context.Context) *time.Location {
	ws, err := st.q.FindWorkspace(ctx, st.run.WorkspaceID)
	if err != nil {
		return time.UTC
	}

	loc, err := time.LoadLocation(ws.Timezone)
	if err != nil {
		return time.UTC
	}

	return loc
}

/*
resumeWait goes on when the time is up, or takes the interrupted path
when the customer or a teammate wrote first.
*/
func resumeWait(_ context.Context, _ *state, s workflowspec.Step, in Input) (outcome, error) {
	if in.Kind == InputTimer {
		return outcome{outcome: outcomeElapsed}, nil
	}

	source := interruptCustomer
	if in.Kind == InputTeammateReply {
		source = interruptTeammate
	}

	w := settings[workflowspec.WaitSettings](s)
	if len(w.InterruptOn) > 0 && !slices.Contains(w.InterruptOn, source) {
		return outcome{outcome: outcomeElapsed}, nil
	}

	out := fixedOrEnd(s, workflowspec.OutcomeInterrupted, EndInterrupted)
	out.detail = map[string]any{"by": source}

	return out, nil
}
