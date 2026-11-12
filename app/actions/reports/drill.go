/*
|--------------------------------------------------------------------------
| Drill-down
|--------------------------------------------------------------------------
|
| Every number opens the conversations behind it: the same facts, the
| same date, the same filters and the same access, listed with the
| value each added, slowest or newest first. A list stops at ten
| thousand conversations; past that a narrower filter tells more.
|
*/

package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
The limits of a drill-down list.
*/
const (
	MaxDrill     = 10_000
	MaxDrillPage = 100
)

/*
DrillRow is one conversation behind a number, with the value it added:
seconds for a duration, the score for a rating, nil for a count.
*/
type DrillRow struct {
	ConversationID   ids.UUID
	Number           int64
	Subject          *string
	Status           string
	Channel          string
	StartedAt        time.Time
	ClosedAt         *time.Time
	AssigneeMemberID *ids.UUID
	Value            *float64
}

/*
DrillPage is one page of a drill-down and how many there are, up to
MaxDrill.
*/
type DrillPage struct {
	Rows   []DrillRow
	Total  int64
	Capped bool
}

/*
drill is how a metric picks its conversations: the date it is counted
by, any condition beyond the filters, the value shown and the order.
*/
type drill struct {
	dated string
	extra func(s *statement, q Query) string
	value string
	order string
}

/*
cond returns a fixed condition.
*/
func cond(sql string) func(*statement, Query) string {
	return func(*statement, Query) string { return sql }
}

/*
drills returns every metric that lists its conversations.
*/
func drills(q Query) map[string]drill {
	resp, res := seconds(q, "f.first_response_seconds"), seconds(q, "f.resolution_seconds")
	open := cond("f.status = 'open'")

	return map[string]drill{
		"new_conversations":          {dated: "f.started_at"},
		"closed":                     {dated: "f.closed_at", value: res},
		"first_response":             {dated: "f.first_response_at", value: resp, extra: cond(resp + " IS NOT NULL")},
		"first_response_within_hour": {dated: "f.first_response_at", value: resp, extra: cond(resp + " <= 3600")},
		"first_answer": {
			dated: "f.first_answer_at", value: seconds(q, "f.first_answer_seconds"),
			extra: cond("f.first_answer_seconds IS NOT NULL"),
		},
		"reply_time":               {extra: repliedIn, value: "f.human_replies"},
		"replied":                  {extra: repliedIn, value: "f.human_replies"},
		"resolution":               {dated: "f.closed_at", value: res},
		"first_close":              {dated: "f.first_closed_at", value: seconds(q, "f.first_close_seconds")},
		"first_contact_resolution": {dated: "f.closed_at", value: res, extra: cond("f.first_contact_resolved")},
		"reopen_rate":              {dated: "f.closed_at", value: "f.reopens", extra: cond("f.reopens > 0")},
		"closed_by_teammate": {
			dated: "f.closed_at", value: res, extra: cond("f.closed_by_member_id IS NOT NULL"),
		},
		"csat":           {dated: "f.rated_at", value: "f.rating", order: "f.rating ASC"},
		"ratings":        {dated: "f.rated_at", value: "f.rating", order: "f.rating ASC"},
		"low_ratings":    {dated: "f.rated_at", value: "f.rating", extra: cond("f.rating <= 2")},
		"ai_involvement": {dated: "f.started_at", extra: cond("f.ai_involved")},
		"ai_handoff": {
			dated: "f.started_at", extra: cond("f.ai_involved AND f.ai_handed_off_at IS NOT NULL"),
		},
		"ai_resolution": {dated: "f.started_at", value: "f.ai_cost_micros", extra: aiResolvedCond("")},
		"ai_confirmed": {
			dated: "f.started_at", value: "f.ai_cost_micros", extra: aiResolvedCond(" AND f.ai_confirmed"),
		},
		"ai_assumed": {
			dated: "f.started_at", value: "f.ai_cost_micros",
			extra: aiResolvedCond(" AND NOT f.ai_confirmed"),
		},
		"open":       {extra: open, order: "f.started_at ASC"},
		"unassigned": {extra: cond("f.status = 'open' AND f.assignee_member_id IS NULL"), order: "f.started_at ASC"},
		"awaiting_first_response": {
			order: "f.started_at ASC",
			extra: cond("f.status = 'open' AND f.first_response_at IS NULL AND f.customer_messages > 0"),
		},
		"snoozed": {extra: cond("f.status = 'snoozed'"), order: "f.started_at ASC"},
	}
}

/*
repliedIn picks conversations a teammate replied to in the range.
*/
func repliedIn(s *statement, q Query) string {
	return fmt.Sprintf("EXISTS (SELECT 1 FROM conversation_reply_facts r "+
		"WHERE r.conversation_id = f.conversation_id "+
		"AND r.replied_at >= %s AND r.replied_at < %s)", s.arg(q.Start), s.arg(q.End))
}

/*
aiResolvedCond picks the AI's resolutions, narrowed by more.
*/
func aiResolvedCond(more string) func(*statement, Query) string {
	return func(s *statement, q Query) string {
		return reader{q: q}.aiResolved(s) + more
	}
}

/*
Conversations lists the conversations behind one of a report's numbers.
*/
func (r *Reports) Conversations(
	ctx context.Context, m authz.Membership, metric string, in DrillInput,
) (DrillPage, error) {
	q, err := NewQuery(m, in.Filters, r.Clock.Now())
	if err != nil {
		return DrillPage{}, err
	}

	d, ok := drills(q)[metric]
	if !ok {
		return DrillPage{}, exceptions.ErrReportMetric
	}

	var out DrillPage

	err = r.read(ctx, m, func(ctx context.Context, tx *database.Tx) error {
		out, err = d.list(ctx, tx, q, in)

		return err
	})

	return out, err
}

/*
DrillInput is the filters of the report and the page asked for.
*/
type DrillInput struct {
	Filters Filters
	Offset  int
	Limit   int
}
