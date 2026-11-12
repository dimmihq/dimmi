/*
|--------------------------------------------------------------------------
| Team and Teammates
|--------------------------------------------------------------------------
|
| What each teammate did in the range: the conversations they replied
| to and their replies, the conversations they held when they closed,
| their first responses and reply times, how long they held waiting
| conversations open, their CSAT with how many ratings it rests on, and
| what they hold open now. Teams get the closes they held and how long
| those took.
|
*/

package reports

import (
	"cmp"
	"context"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
Team is the team and teammates report.
*/
type Team struct{}

/*
Describe says what the report is.
*/
func (Team) Describe() Description {
	return Description{
		Key: "team", Title: "Team and teammates", Order: 40,
		Summary: "Each teammate's replies, closes, speed, CSAT and open load.",
	}
}

/*
Metrics returns the headline numbers.
*/
func (Team) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	r := reader{tx: tx, q: q}

	active, err := r.one(ctx, figures{
		from: fromMembers, dated: "mf.last_replied_at",
		cols: "count(DISTINCT mf.member_id)::float8, COALESCE(sum(mf.replies), 0)::float8, " +
			"count(DISTINCT mf.conversation_id)::float8",
	}, 3)
	if err != nil {
		return nil, err
	}

	replies, err := r.replyTimes(ctx, "")
	if err != nil {
		return nil, err
	}

	closes, err := r.one(ctx, figures{
		from: fromFacts, dated: "f.closed_at",
		cols: "count(*) FILTER (WHERE f.closed_by_member_id IS NOT NULL)::float8",
	}, 1)
	if err != nil {
		return nil, err
	}

	return []Metric{
		countMetric("teammates_active", "Teammates who replied", active.count(0)),
		countMetric("replied", "Conversations replied to", active.count(2)),
		{Key: "replies", Label: "Replies", Unit: UnitCount, Value: active.at(1), Sample: active.count(1)},
		countMetric("closed_by_teammate", "Closed by a teammate", closes.count(0)),
		durationMetric("reply_time", "Reply time", durationOf(only(replies, 4), 0)),
	}, nil
}

/*
Charts returns the teammates and teams tables.
*/
func (t Team) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	return sequential(ctx, tx, q, t.ChartParts())
}

/*
ChartParts returns the two tables as parts that can be read at once.
*/
func (Team) ChartParts() []ChartPart {
	return []ChartPart{
		func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			n, err := loadNames(ctx, tx, q.Membership.Workspace.ID)
			if err != nil {
				return nil, err
			}

			people, err := reader{tx: tx, q: q}.teammates(ctx, n)

			return []Chart{people}, err
		},
		func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			n, err := loadNames(ctx, tx, q.Membership.Workspace.ID)
			if err != nil {
				return nil, err
			}

			r := reader{tx: tx, q: q}
			teams, err := breakdown{
				key: "teams", title: "Teams", group: "f.closed_by_team_id", drill: "closed",
				columns: append([]Column{{Key: "closed", Label: "Closed", Unit: UnitCount}},
					durationColumns("Resolved")[1:]...),
				read: r.closedFigures,
				pick: func(v values) []*float64 { return []*float64{v.at(0), v.at(4), v.at(5)} },
			}.table(ctx, n)

			return []Chart{teams}, err
		},
	}
}

/*
teammateColumns are the columns of the teammates table, each read by
its own grouped query.
*/
func teammateColumns() []Column {
	return []Column{
		{Key: "replied", Label: "Conversations replied", Unit: UnitCount},
		{Key: "replies", Label: "Replies", Unit: UnitCount},
		{Key: "closed", Label: "Closed", Unit: UnitCount},
		{Key: "first_response", Label: "First response", Unit: UnitSeconds},
		{Key: "reply_time", Label: "Reply time", Unit: UnitSeconds},
		{Key: "held_open", Label: "Time held open", Unit: UnitSeconds},
		{Key: "csat", Label: "CSAT", Unit: UnitPercent},
		{Key: "ratings", Label: "Ratings", Unit: UnitCount},
		{Key: "open", Label: "Open now", Unit: UnitCount},
	}
}

/*
teammates builds the table of each teammate's figures.
*/
func (r reader) teammates(ctx context.Context, n names) (Chart, error) {
	cells, err := r.teammateCells(ctx)
	if err != nil {
		return Chart{}, err
	}

	out := Chart{
		Key: "teammates", Title: "Teammates", Kind: ChartTable, Axis: AxisCategory, Drill: "replied",
		Columns: teammateColumns(),
	}
	for key, vals := range cells {
		if key == "" {
			continue
		}

		out.Rows = append(out.Rows, n.row(groupMember, key, vals))
	}

	slices.SortFunc(out.Rows, func(a, b Row) int {
		if c := cmp.Compare(first(b), first(a)); c != 0 {
			return c
		}

		return cmp.Compare(a.Label, b.Label)
	})

	return out, nil
}

/*
teammateCells reads every part of the teammates table into one row of
cells per teammate.
*/
func (r reader) teammateCells(ctx context.Context) (map[string][]*float64, error) {
	cells := map[string][]*float64{}
	for _, part := range r.teammateParts() {
		rows, err := r.read(ctx, part.f, part.group, len(part.cols))
		if err != nil {
			return nil, err
		}

		for _, row := range rows {
			if _, ok := cells[row.key]; !ok {
				cells[row.key] = make([]*float64, len(teammateColumns()))
			}

			part.fill(cells[row.key], row)
		}
	}

	return cells, nil
}

/*
teammatePart is one grouped query of the teammates table and the
columns it fills.
*/
type teammatePart struct {
	f     figures
	group string
	cols  []int
}

/*
fill copies a row's figures into the cells of the columns they fill.
*/
func (p teammatePart) fill(cells []*float64, row values) {
	for i, col := range p.cols {
		if col >= 0 {
			cells[col] = row.at(i)
		}
	}
}

/*
teammateParts are the queries the teammates table is read from.
*/
func (r reader) teammateParts() []teammatePart {
	q := r.q
	open := figures{from: fromFacts, cols: "count(*)::float8", extra: "f.status <> 'closed'"}

	return []teammatePart{
		{figures{
			from: fromMembers, dated: "mf.last_replied_at", extra: "mf.replies > 0",
			cols: "count(*)::float8, sum(mf.replies)::float8, sum(mf.assigned_open_seconds)::float8",
		}, "mf.member_id", []int{0, 1, 5}},
		{figures{from: fromFacts, dated: "f.closed_at", cols: "count(*)::float8"}, "f.closed_by_member_id", []int{2}},
		{
			durationFigures(fromFacts, "f.first_response_at", seconds(q, "f.first_response_seconds")),
			"f.first_responder_id",
			[]int{-1, 3, -1, -1},
		},
		{durationFigures(fromReplies, "r.replied_at", seconds(q, "r.seconds")), "r.member_id", []int{-1, 4, -1, -1}},
		{figures{
			from: fromFacts, dated: "f.rated_at", extra: "f.rated_member_id IS NOT NULL",
			cols: "(count(*) FILTER (WHERE f.rating >= 4) * 100.0 / count(*))::float8, count(*)::float8",
		}, "f.rated_member_id", []int{6, 7}},
		{open, "f.assignee_member_id", []int{8}},
	}
}
