/*
|--------------------------------------------------------------------------
| Telling The Customer When The Team Replies
|--------------------------------------------------------------------------
|
| A reply time step sends one message with the {{reply_time}} variable
| filled in. Outside office hours the answer is when the team is back,
| in the schedule's own timezone. Inside them it starts from how fast
| the team answered first messages over the last two weeks, an hour
| when there is no history, and grows when nobody is available or when
| many conversations wait per available teammate. The answer is a few
| plain words, never a promise to the minute.
|
*/

package workflows

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
How a reply time is estimated: the history it reads, the reply time
without one, the wait when nobody is available, and the queue per
teammate that counts as busy.
*/
const (
	replyHistory     = 14 * 24 * time.Hour
	replyFallback    = time.Hour
	replyNobodyThere = 3 * time.Hour
	replyBusyQueue   = 5
)

/*
replyTimeVariable finds the reply time variable in a message.
*/
func replyTimeVariable() *regexp.Regexp {
	return regexp.MustCompile(`\{\{\s*reply_time\s*(\|[^}]*)?\}\}`)
}

/*
runReplyTime tells the customer when to expect a reply.
*/
func runReplyTime(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	r := settings[workflowspec.ReplyTimeSettings](s)

	phrase, open, err := st.replyTime(ctx, r)
	if err != nil {
		return outcome{}, err
	}

	text := r.Text
	if text == "" {
		text = workflowspec.DefaultReplyTimeText
	}

	text, err = st.render(ctx, replyTimeVariable().ReplaceAllLiteralString(text, phrase))
	if err != nil {
		return outcome{}, err
	}

	out, err := st.send(ctx, s, text, "")
	if out.detail == nil {
		out.detail = map[string]any{}
	}

	out.detail["reply_time"], out.detail["open"] = phrase, open

	return out, err
}

/*
replyTime is the reply time in words and whether the team is open.
*/
func (st *state) replyTime(ctx context.Context, r workflowspec.ReplyTimeSettings) (string, bool, error) {
	now := st.now()

	sched, err := st.e.hours(ctx, st.q, placeOf(st.conv), r.ScheduleID)
	if err != nil && !scheduleProblem(err) {
		return "", false, err
	}

	if err == nil && !sched.IsOpen(now) {
		if next, err := sched.NextOpen(now); err == nil {
			return backAt(now, next, sched), false, nil
		}
	}

	params := models.ReplyTimeFactsParams{WorkspaceID: st.conv.WorkspaceID, Since: now.Add(-replyHistory)}
	if team, ok := st.decode(models.PrefixTeam, r.TeamID); ok {
		params.TeamID = &team
	}

	facts, err := st.q.ReplyTimeFacts(ctx, params)
	if err != nil {
		return "", true, fmt.Errorf("read reply time facts: %w", err)
	}

	return withinWords(estimate(facts)), true, nil
}

/*
estimate is how long a reply likely takes while the team is open.
*/
func estimate(f models.ReplyTimeFactsRow) time.Duration {
	typical := time.Duration(f.MedianSeconds * float64(time.Second))
	if typical <= 0 {
		typical = replyFallback
	}

	switch {
	case f.Available == 0:
		typical = max(typical, replyNobodyThere)
	case f.Waiting/f.Available >= replyBusyQueue:
		typical *= 2
	}

	return typical
}

/*
withinWords says a duration as a customer reads it.
*/
func withinWords(d time.Duration) string {
	switch {
	case d <= 10*time.Minute:
		return "in a few minutes"
	case d <= time.Hour:
		return "within an hour"
	case d <= 4*time.Hour:
		return "in a few hours"
	}

	return "within a day"
}

/*
backAt says when a closed team is back: later today, tomorrow, on a
weekday this week, or on a date.
*/
func backAt(now, next time.Time, sched *officehours.Schedule) string {
	loc := sched.Location()
	from, at := now.In(loc), next.In(loc)
	clock := at.Format("15:04")

	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc) }
	days := int(day(at).Sub(day(from)).Hours() / 24)

	switch {
	case days <= 0:
		return "later today after " + clock
	case days == 1:
		return "tomorrow after " + clock
	case days < 7:
		return "on " + at.Weekday().String() + " after " + clock
	}

	return "on " + at.Format("2 January") + " after " + clock
}
