/*
|--------------------------------------------------------------------------
| Resolution
|--------------------------------------------------------------------------
|
| How conversations end: how long they take to close, how often they
| close on first contact, how often they come back, each day, in time
| buckets, and split by channel, inbox, team and priority.
|
*/

package reports

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
Resolution is the resolution report.
*/
type Resolution struct{}

/*
Describe says what the report is.
*/
func (Resolution) Describe() Description {
	return Description{
		Key: "resolution", Title: "Resolution", Order: 30,
		Summary: "Time to close, first contact resolution and reopens.",
	}
}

/*
Metrics returns the headline numbers.
*/
func (Resolution) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	r := reader{tx: tx, q: q}

	closed, err := r.closedFigures(ctx, "")
	if err != nil {
		return nil, err
	}

	firstClose, err := r.read(ctx, durationFigures(fromFacts, "f.first_closed_at", seconds(q, "f.first_close_seconds")),
		"", 4)
	if err != nil {
		return nil, err
	}

	c := only(closed, 7)

	return []Metric{
		countMetric("closed", "Closed", c.count(0)),
		durationMetric("resolution", "Resolution time", durationOf(c, 3)),
		durationMetric("first_close", "Time to first close", durationOf(only(firstClose, 4), 0)),
		rateMetric("first_contact_resolution", "First contact resolution", c.count(2), c.count(0)),
		rateMetric("reopen_rate", "Reopen rate", c.count(1), c.count(0)),
	}, nil
}

/*
Charts returns the charts.
*/
func (rs Resolution) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	return sequential(ctx, tx, q, rs.ChartParts())
}

/*
ChartParts returns the charts as parts that can be read at once.
*/
func (Resolution) ChartParts() []ChartPart {
	parts := []ChartPart{
		func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			return reader{tx: tx, q: q}.resolutionDaily(ctx)
		},
		func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			c, err := reader{tx: tx, q: q}.histogram(ctx, buckets{
				key: "resolution_buckets", title: "Resolution times", dated: "f.closed_at",
				column: seconds(q, "f.resolution_seconds"), drill: "resolution",
			})

			return []Chart{c}, err
		},
	}

	return append(parts, splitParts(func(r reader) groupRead {
		return func(ctx context.Context, g string) ([]values, error) {
			rows, err := r.closedFigures(ctx, g)
			for i := range rows {
				rows[i].nums = rows[i].nums[3:]
			}

			return rows, err
		}
	}, split{drill: "resolution", title: "Resolution time by %s", count: "Closed"})...)
}

/*
resolutionDaily is resolution time and how conversations closed, each
day.
*/
func (r reader) resolutionDaily(ctx context.Context) ([]Chart, error) {
	q := r.q

	closed, err := r.closedFigures(ctx, groupDay)
	if err != nil {
		return nil, err
	}

	return []Chart{
		{
			Key: "resolution_daily", Title: "Resolution time", Kind: ChartLine, Unit: UnitSeconds, Axis: AxisDay,
			Drill: "resolution", Series: []Series{
				daily(q, Series{Key: "median", Label: "Median"}, closed, at(4)),
				daily(q, Series{Key: "p90", Label: "90th percentile"}, closed, at(5)),
			},
		},
		{
			Key: "outcomes_daily", Title: "Closed conversations", Kind: ChartStacked, Unit: UnitCount, Axis: AxisDay,
			Series: []Series{
				daily(q, Series{Key: "first_contact", Label: "First contact", Drill: "first_contact_resolution"},
					closed, at(2)),
				daily(q, Series{Key: "reopened", Label: "Reopened", Drill: "reopen_rate"}, closed, at(1)),
				daily(q, Series{Key: "other", Label: "Other", Drill: "closed"}, closed, func(v values) *float64 {
					return num(v.count(0) - v.count(1) - v.count(2))
				}),
			},
		},
	}, nil
}
