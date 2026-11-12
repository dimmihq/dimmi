/*
|--------------------------------------------------------------------------
| Satisfaction
|--------------------------------------------------------------------------
|
| How customers rated their conversations, dated by when they rated:
| CSAT is the ratings of four or five out of all ratings. Each rating is
| credited to the teammate it rated, or to dimmi when no teammate
| replied, and every split shows how many ratings it rests on.
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
Satisfaction is the satisfaction report.
*/
type Satisfaction struct{}

/*
Describe says what the report is.
*/
func (Satisfaction) Describe() Description {
	return Description{
		Key: "satisfaction", Title: "Satisfaction", Order: 60,
		Summary: "CSAT, the spread of ratings and who earned them.",
	}
}

/*
Metrics returns the headline numbers.
*/
func (Satisfaction) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	r := reader{tx: tx, q: q}

	v, err := r.ratingFigures(ctx, "")
	if err != nil {
		return nil, err
	}

	o := only(v, 3)

	low, err := r.one(ctx, figures{
		from: fromFacts, dated: "f.rated_at", cols: "count(*)::float8", extra: "f.rating <= 2",
	}, 1)
	if err != nil {
		return nil, err
	}

	return []Metric{
		rateMetric("csat", "CSAT", o.count(1), o.count(0)),
		countMetric("ratings", "Ratings", o.count(0)),
		{
			Key: "average_rating", Label: "Average rating", Unit: UnitScore, Value: o.at(2), Sample: o.count(0),
			Drill: "ratings",
		},
		countMetric("low_ratings", "Ratings of 1 or 2", low.count(0)),
	}, nil
}

/*
Charts returns the charts.
*/
func (Satisfaction) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	r := reader{tx: tx, q: q}

	byDay, err := r.ratingFigures(ctx, groupDay)
	if err != nil {
		return nil, err
	}

	dist, err := r.ratingSpread(ctx)
	if err != nil {
		return nil, err
	}

	n, err := loadNames(ctx, tx, q.Membership.Workspace.ID)
	if err != nil {
		return nil, err
	}

	tables, err := r.csatTables(ctx, n)

	return append([]Chart{
		{
			Key: "csat_daily", Title: "CSAT", Kind: ChartLine, Unit: UnitPercent, Axis: AxisDay, Drill: "ratings",
			Series: []Series{daily(q, Series{Key: "csat", Label: "CSAT"}, byDay, percentOf(1, 0))},
		},
		{
			Key: "rating_spread", Title: "Ratings", Kind: ChartBar, Unit: UnitCount, Axis: AxisCategory,
			Drill: "ratings", Series: []Series{dist},
		},
	}, tables...), err
}

/*
ratingSpread counts the ratings given in the range by score.
*/
func (r reader) ratingSpread(ctx context.Context) (Series, error) {
	var cols strings.Builder
	for score := 1; score <= 5; score++ {
		if score > 1 {
			cols.WriteString(", ")
		}

		fmt.Fprintf(&cols, "count(*) FILTER (WHERE f.rating = %d)::float8", score)
	}

	spread, err := r.one(ctx, figures{from: fromFacts, dated: "f.rated_at", cols: cols.String()}, 5)
	if err != nil {
		return Series{}, err
	}

	dist := Series{Key: "ratings", Label: "Ratings"}
	for score := 1; score <= 5; score++ {
		dist.Points = append(dist.Points, Point{X: fmt.Sprint(score), Y: spread.at(score - 1)})
	}

	return dist, nil
}

/*
csatTables returns CSAT by teammate, dimmi taking the ratings no
teammate earned, and CSAT by channel.
*/
func (r reader) csatTables(ctx context.Context, n names) ([]Chart, error) {
	csatCols := []Column{
		{Key: "ratings", Label: "Ratings", Unit: UnitCount}, {Key: "csat", Label: "CSAT", Unit: UnitPercent},
	}
	pick := func(v values) []*float64 { return []*float64{v.at(0), ratio(v.count(1), v.count(0))} }

	people, err := breakdown{
		key: "csat_by_teammate", title: "CSAT by teammate", group: "f.rated_member_id",
		drill: "ratings", columns: csatCols, read: r.ratingFigures, pick: pick,
	}.table(ctx, n)
	if err != nil {
		return nil, err
	}

	for i := range people.Rows {
		people.Rows[i] = n.row(groupMember, people.Rows[i].Key, people.Rows[i].Values)
		if people.Rows[i].Key == "none" {
			people.Rows[i].Label = "dimmi"
		}
	}

	channels, err := breakdown{
		key: "csat_by_channel", title: "CSAT by channel", group: groupChannel,
		drill: "ratings", columns: csatCols, read: r.ratingFigures, pick: pick,
	}.table(ctx, n)

	return []Chart{people, channels}, err
}
