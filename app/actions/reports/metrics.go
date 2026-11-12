/*
|--------------------------------------------------------------------------
| Shared Figures
|--------------------------------------------------------------------------
|
| The figures several reports show, each dated by when it ends: a new
| conversation by its start, a close by the last close, a first
| response by the reply, a turn by its reply and a rating by when it was
| given. The AI's figures follow the conversations started in the range,
| so its rates divide like by like. A duration is its median, with the
| 90th percentile and the mean beside it.
|
*/

package reports

import (
	"context"
	"fmt"
	"time"
)

/*
The groups figures can be split by. The day group is the local day of
the dated column, the hour group its local weekday and hour as "1-09".
*/
const (
	groupDay      = "day"
	groupHour     = "hour"
	groupChannel  = "f.channel"
	groupInbox    = "f.inbox_id"
	groupTeam     = "f.assignee_team_id"
	groupMember   = "f.assignee_member_id"
	groupPriority = "f.priority"
)

/*
silence is how long a customer stays quiet after an AI answer before
the conversation counts as resolved by it.
*/
const silence = 24 * time.Hour

/*
durationFigures reads a duration column dated by another, from the
facts or a table joined to them.
*/
func durationFigures(from, dated, column string) figures {
	return figures{from: from, dated: dated, cols: durationStats(column), extra: column + " IS NOT NULL"}
}

/*
durationOf reads a duration's stats from four figures at the index.
*/
func durationOf(v values, i int) Duration {
	return Duration{Count: v.count(i), Median: v.at(i + 1), P90: v.at(i + 2), Mean: v.at(i + 3)}
}

/*
durationMetric is a headline duration.
*/
func durationMetric(key, label string, d Duration) Metric {
	return Metric{
		Key: key, Label: label, Unit: UnitSeconds, Value: d.Median, P90: d.P90, Mean: d.Mean, Sample: d.Count,
		Drill: key,
	}
}

/*
countMetric is a headline count.
*/
func countMetric(key, label string, n int64) Metric {
	return Metric{Key: key, Label: label, Unit: UnitCount, Value: num(n), Sample: n, Drill: key}
}

/*
rateMetric is a headline percent of a whole.
*/
func rateMetric(key, label string, part, whole int64) Metric {
	return Metric{Key: key, Label: label, Unit: UnitPercent, Value: ratio(part, whole), Sample: whole, Drill: key}
}

/*
firstResponse reads first responses.
*/
func (r reader) firstResponse(ctx context.Context, group string) ([]values, error) {
	return r.read(ctx, durationFigures(fromFacts, "f.first_response_at", seconds(r.q, "f.first_response_seconds")),
		group, 4)
}

/*
replyTimes reads answered customer turns.
*/
func (r reader) replyTimes(ctx context.Context, group string) ([]values, error) {
	return r.read(ctx, durationFigures(fromReplies, "r.replied_at", seconds(r.q, "r.seconds")), group, 4)
}

/*
closedFigures reads the closes in the range: how many, how many had
reopened, how many were resolved on first contact, and resolution.
*/
func (r reader) closedFigures(ctx context.Context, group string) ([]values, error) {
	col := seconds(r.q, "f.resolution_seconds")

	return r.read(ctx, figures{from: fromFacts, dated: "f.closed_at", cols: fmt.Sprintf(
		"count(*)::float8, count(*) FILTER (WHERE f.reopens > 0)::float8, "+
			"count(*) FILTER (WHERE f.first_contact_resolved)::float8, %s", durationStats(col),
	)}, group, 7)
}

/*
startedFigures reads the conversations started in the range: how many,
how many the AI answered in, and of those how many it handed off, how
many it resolved, how many confirmed, and their cost.
*/
func (r reader) startedFigures(ctx context.Context, group string) ([]values, error) {
	var s statement
	resolved := r.aiResolved(&s)

	f := figures{from: fromFacts, dated: "f.started_at", cols: fmt.Sprintf(
		"count(*)::float8, count(*) FILTER (WHERE f.ai_involved)::float8, "+
			"count(*) FILTER (WHERE f.ai_involved AND f.ai_handed_off_at IS NOT NULL)::float8, "+
			"count(*) FILTER (WHERE %[1]s)::float8, count(*) FILTER (WHERE %[1]s AND f.ai_confirmed)::float8, "+
			"COALESCE(sum(f.ai_cost_micros), 0)::float8", resolved,
	)}

	return r.bound(s).read(ctx, f, group, 6)
}

/*
aiResolved is the condition of an AI resolution: the AI answered, no
teammate replied, it handed nothing off, and the customer confirmed it,
the conversation closed or the customer stayed quiet a day after the
last answer.
*/
func (r reader) aiResolved(s *statement) string {
	return "(f.ai_involved AND f.human_replies = 0 AND f.ai_handed_off_at IS NULL AND (f.ai_confirmed " +
		"OR f.closed_at IS NOT NULL OR (f.ai_last_answer_at >= COALESCE(f.last_customer_message_at, f.started_at) " +
		"AND f.ai_last_answer_at < " + s.arg(r.q.Now.Add(-silence)) + ")))"
}

/*
ratingFigures reads the ratings given in the range: how many, how many
were four or five, and the mean score.
*/
func (r reader) ratingFigures(ctx context.Context, group string) ([]values, error) {
	return r.read(ctx, figures{from: fromFacts, dated: "f.rated_at", cols: "count(*)::float8, " +
		"count(*) FILTER (WHERE f.rating >= 4)::float8, avg(f.rating)::float8"}, group, 3)
}

/*
bound returns the reader for figures whose columns already bound values
to the statement.
*/
func (r reader) bound(s statement) reader {
	r.pre = s.args

	return r
}
