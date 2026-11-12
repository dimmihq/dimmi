/*
|--------------------------------------------------------------------------
| Responsiveness
|--------------------------------------------------------------------------
|
| How fast customers hear back: the first response by a teammate, the
| first answer by anyone, the AI included, and the reply to each later
| customer turn, each day and split by channel, inbox, team and
| priority.
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
Responsiveness is the responsiveness report.
*/
type Responsiveness struct{}

/*
Describe says what the report is.
*/
func (Responsiveness) Describe() Description {
	return Description{
		Key: "responsiveness", Title: "Responsiveness", Order: 20,
		Summary: "First response and reply times, and where they are slow.",
	}
}

/*
Metrics returns the headline numbers.
*/
func (Responsiveness) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	r := reader{tx: tx, q: q}

	first, err := r.firstResponse(ctx, "")
	if err != nil {
		return nil, err
	}

	answer, err := r.read(ctx,
		durationFigures(fromFacts, "f.first_answer_at", seconds(q, "f.first_answer_seconds")), "", 4)
	if err != nil {
		return nil, err
	}

	replies, err := r.replyTimes(ctx, "")
	if err != nil {
		return nil, err
	}

	within, err := r.one(ctx, figures{
		from: fromFacts, dated: "f.first_response_at", extra: "f.first_response_seconds IS NOT NULL",
		cols: fmt.Sprintf("count(*)::float8, count(*) FILTER (WHERE %s <= 3600)::float8",
			seconds(q, "f.first_response_seconds")),
	}, 2)
	if err != nil {
		return nil, err
	}

	return []Metric{
		durationMetric("first_response", "First response", durationOf(only(first, 4), 0)),
		durationMetric("first_answer", "First answer, AI included", durationOf(only(answer, 4), 0)),
		durationMetric("reply_time", "Reply time", durationOf(only(replies, 4), 0)),
		rateMetric("first_response_within_hour", "First responses within an hour", within.count(1), within.count(0)),
	}, nil
}

/*
Charts returns the charts.
*/
func (rs Responsiveness) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	return sequential(ctx, tx, q, rs.ChartParts())
}

/*
ChartParts returns the charts as parts that can be read at once: the
daily lines, the time buckets and each breakdown.
*/
func (Responsiveness) ChartParts() []ChartPart {
	parts := []ChartPart{
		func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			r := reader{tx: tx, q: q}

			first, err := r.firstResponse(ctx, groupDay)
			if err != nil {
				return nil, err
			}

			c := Chart{Key: "first_response_daily", Title: "First response", Drill: "first_response"}

			return []Chart{dailyDuration(c, q, first)}, nil
		},
		func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			r := reader{tx: tx, q: q}

			replies, err := r.replyTimes(ctx, groupDay)
			if err != nil {
				return nil, err
			}

			c := Chart{Key: "reply_time_daily", Title: "Reply time", Drill: "reply_time"}

			return []Chart{dailyDuration(c, q, replies)}, nil
		},
		func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			c, err := reader{tx: tx, q: q}.histogram(ctx, buckets{
				key: "first_response_buckets", title: "First response times", dated: "f.first_response_at",
				column: seconds(q, "f.first_response_seconds"), drill: "first_response",
			})

			return []Chart{c}, err
		},
	}

	return append(parts, splitParts(func(r reader) groupRead { return r.firstResponse }, split{
		drill: "first_response", title: "First response by %s", count: "Responses",
	})...)
}

/*
dailyDuration fills the chart, named by its key, title and drill, with a
duration's median and 90th percentile each day, from grouped duration
figures.
*/
func dailyDuration(c Chart, q Query, rows []values) Chart {
	c.Kind, c.Unit, c.Axis = ChartLine, UnitSeconds, AxisDay
	c.Series = []Series{
		daily(q, Series{Key: "median", Label: "Median"}, rows, at(1)),
		daily(q, Series{Key: "p90", Label: "90th percentile"}, rows, at(2)),
	}

	return c
}

/*
splitParts returns one part per breakdown of a split.
*/
func splitParts(read func(reader) groupRead, sp split) []ChartPart {
	keys := []string{"channel", "inbox", "team", "priority"}
	out := make([]ChartPart, 0, len(keys))

	for _, key := range keys {
		one := sp
		one.only = key
		out = append(out, func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			r := reader{tx: tx, q: q}

			return r.splits(ctx, read(r), one)
		})
	}

	return out
}

/*
split names a duration's breakdowns: the drill-down they open, the
title with a place for the group, and the label of the count column.
*/
type split struct {
	drill, title, count string
	only                string
}

/*
splits returns a duration's breakdowns by channel, inbox, team and
priority.
*/
func (r reader) splits(ctx context.Context, read groupRead, sp split) ([]Chart, error) {
	n, err := loadNames(ctx, r.tx, r.q.Membership.Workspace.ID)
	if err != nil {
		return nil, err
	}

	groups := []struct{ key, group, name string }{
		{"channel", groupChannel, "channel"},
		{"inbox", groupInbox, "inbox"},
		{"team", groupTeam, "team"},
		{"priority", groupPriority, "priority"},
	}

	out := make([]Chart, 0, len(groups))
	for _, g := range groups {
		if sp.only != "" && sp.only != g.key {
			continue
		}

		c, err := breakdown{
			key: sp.drill + "_by_" + g.key, title: fmt.Sprintf(sp.title, g.name), group: g.group, drill: sp.drill,
			columns: durationColumns(sp.count), read: read, pick: durationValues(0),
		}.table(ctx, n)
		if err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	return out, nil
}

/*
buckets describes a histogram: its key and title, the column it is
dated by, the duration column counted and the drill-down it opens.
*/
type buckets struct {
	key, title, dated, column, drill string
}

/*
histogram counts a duration in buckets from minutes to over a week.
*/
func (r reader) histogram(ctx context.Context, b buckets) (Chart, error) {
	column := b.column

	bounds := []struct {
		label string
		upTo  int
	}{
		{"Under 5 min", 300},
		{"5–15 min", 900},
		{"15–60 min", 3600},
		{"1–4 h", 4 * 3600},
		{"4–24 h", 24 * 3600},
		{"1–3 days", 3 * 86400},
		{"3–7 days", 7 * 86400},
		{"Over 7 days", 0},
	}

	var cols strings.Builder
	low := 0
	for i, bound := range bounds {
		if i > 0 {
			cols.WriteString(", ")
		}

		cond := fmt.Sprintf("%s >= %d", column, low)
		if bound.upTo > 0 {
			cond += fmt.Sprintf(" AND %s < %d", column, bound.upTo)
		}

		fmt.Fprintf(&cols, "count(*) FILTER (WHERE %s)::float8", cond)
		low = bound.upTo
	}

	v, err := r.one(ctx, figures{
		from: fromFacts, dated: b.dated, cols: cols.String(), extra: column + " IS NOT NULL",
	}, len(bounds))
	if err != nil {
		return Chart{}, err
	}

	s := Series{Key: "conversations", Label: "Conversations"}
	for i, bound := range bounds {
		s.Points = append(s.Points, Point{X: bound.label, Y: v.at(i)})
	}

	return Chart{
		Key: b.key, Title: b.title, Kind: ChartHistogram, Unit: UnitCount, Axis: AxisCategory, Drill: b.drill,
		Series: []Series{s},
	}, nil
}
