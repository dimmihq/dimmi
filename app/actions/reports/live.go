/*
|--------------------------------------------------------------------------
| Live Workload
|--------------------------------------------------------------------------
|
| What waits right now, whatever the range: open and snoozed
| conversations, those nobody holds, those still waiting for a first
| response and how long the oldest has waited, by inbox, team and
| teammate, and by how long they have waited. The facts follow every
| change within seconds; the page asks again every thirty.
|
*/

package reports

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
Live is the live workload report.
*/
type Live struct{}

/*
Describe says what the report is.
*/
func (Live) Describe() Description {
	return Description{
		Key: "live", Title: "Live workload", Order: 50, Live: true,
		Summary: "What is waiting right now, and who holds it.",
	}
}

/*
waiting is the condition of a conversation still waiting.
*/
const waiting = "f.status <> 'closed'"

/*
Metrics returns the headline numbers.
*/
func (Live) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	r := reader{tx: tx, q: q}

	var s statement
	now := s.arg(q.Now)

	v, err := r.bound(s).read(ctx, figures{from: fromFacts, extra: waiting, cols: fmt.Sprintf(
		"count(*) FILTER (WHERE f.status = 'open')::float8, count(*) FILTER (WHERE f.status = 'snoozed')::float8, "+
			"count(*) FILTER (WHERE f.status = 'open' AND f.assignee_member_id IS NULL)::float8, "+
			"count(*) FILTER (WHERE f.status = 'open' AND f.first_response_at IS NULL "+
			"AND f.customer_messages > 0)::float8, "+
			"extract(epoch FROM %[1]s::timestamptz - min(f.started_at) FILTER (WHERE f.status = 'open'))::float8, "+
			"percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM %[1]s::timestamptz - f.started_at)) "+
			"FILTER (WHERE f.status = 'open')::float8", now,
	)}, "", 6)
	if err != nil {
		return nil, err
	}

	o := only(v, 6)

	return []Metric{
		countMetric("open", "Open", o.count(0)),
		countMetric("unassigned", "Unassigned", o.count(2)),
		countMetric("awaiting_first_response", "Waiting for a first response", o.count(3)),
		countMetric("snoozed", "Snoozed", o.count(1)),
		{Key: "oldest_open", Label: "Oldest open", Unit: UnitSeconds, Value: o.at(4), Drill: "open"},
		{Key: "median_open_age", Label: "Median open age", Unit: UnitSeconds, Value: o.at(5), Drill: "open"},
	}, nil
}

/*
Charts returns where the open conversations are.
*/
func (Live) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	r := reader{tx: tx, q: q}

	n, err := loadNames(ctx, tx, q.Membership.Workspace.ID)
	if err != nil {
		return nil, err
	}

	cols := []Column{
		{Key: "open", Label: "Open", Unit: UnitCount},
		{Key: "unassigned", Label: "Unassigned", Unit: UnitCount},
		{Key: "awaiting", Label: "Waiting for first response", Unit: UnitCount},
	}
	read := func(ctx context.Context, g string) ([]values, error) {
		return r.read(ctx, figures{from: fromFacts, extra: "f.status = 'open'", cols: "count(*)::float8, " +
			"count(*) FILTER (WHERE f.assignee_member_id IS NULL)::float8, " +
			"count(*) FILTER (WHERE f.first_response_at IS NULL AND f.customer_messages > 0)::float8"}, g, 3)
	}
	pick := func(v values) []*float64 { return v.nums }

	var out []Chart
	for _, g := range []struct{ key, title, group string }{
		{"open_by_inbox", "Open by inbox", groupInbox},
		{"open_by_team", "Open by team", groupTeam},
		{"open_by_teammate", "Open by teammate", groupMember},
	} {
		c, err := breakdown{
			key: g.key, title: g.title, group: g.group, drill: "open", columns: cols, read: read,
			pick: pick,
		}.table(ctx, n)
		if err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	ages, err := r.ages(ctx)

	return append([]Chart{ages}, out...), err
}

/*
ages counts the open conversations by how long they have waited.
*/
func (r reader) ages(ctx context.Context) (Chart, error) {
	var s statement
	now := s.arg(r.q.Now)

	labels := []string{"Under 1 h", "1–4 h", "4–24 h", "1–3 days", "3–7 days", "Over 7 days"}
	bounds := []string{"1 hour", "4 hours", "24 hours", "3 days", "7 days"}

	var cols strings.Builder
	newer := ""
	for i, older := range append(bounds, "") {
		if i > 0 {
			cols.WriteString(", ")
		}

		fmt.Fprintf(&cols, "count(*) FILTER (WHERE %s)::float8", waitedBetween(now, newer, older))
		newer = older
	}

	v, err := r.bound(s).read(ctx, figures{from: fromFacts, extra: "f.status = 'open'", cols: cols.String()},
		"", len(labels))
	if err != nil {
		return Chart{}, err
	}

	o := only(v, len(labels))
	series := Series{Key: "open", Label: "Open"}
	for i, l := range labels {
		series.Points = append(series.Points, Point{X: l, Y: o.at(i)})
	}

	return Chart{
		Key: "open_ages", Title: "How long open conversations have waited", Kind: ChartBar, Unit: UnitCount,
		Axis: AxisCategory, Drill: "open", Series: []Series{series},
	}, nil
}

/*
waitedBetween is the condition of a conversation started between two
intervals before now, newer first; an empty interval leaves that side
open.
*/
func waitedBetween(now, newer, older string) string {
	switch {
	case newer == "":
		return fmt.Sprintf("f.started_at > %s::timestamptz - interval '%s'", now, older)
	case older == "":
		return fmt.Sprintf("f.started_at <= %s::timestamptz - interval '%s'", now, newer)
	default:
		return fmt.Sprintf("f.started_at <= %[1]s::timestamptz - interval '%[2]s' AND "+
			"f.started_at > %[1]s::timestamptz - interval '%[3]s'", now, newer, older)
	}
}
