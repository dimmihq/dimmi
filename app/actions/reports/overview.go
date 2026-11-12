/*
|--------------------------------------------------------------------------
| Overview
|--------------------------------------------------------------------------
|
| The numbers a team checks first: new and closed conversations, first
| response, resolution, CSAT and first contact resolution; how many came
| in against how many closed each day, the busiest hours of the week,
| the backlog at the end of each day and where conversations came from.
|
*/

package reports

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Overview is the overview report.
*/
type Overview struct{}

/*
Describe says what the report is.
*/
func (Overview) Describe() Description {
	return Description{
		Key: "overview", Title: "Overview", Order: 10,
		Summary: "Volume, speed, quality and backlog at a glance.",
	}
}

/*
Metrics returns the headline numbers.
*/
func (Overview) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	r := reader{tx: tx, q: q}

	h, err := r.headline(ctx)
	if err != nil {
		return nil, err
	}

	return []Metric{
		countMetric("new_conversations", "New conversations", h.started.count(0)),
		countMetric("closed", "Closed", h.closed.count(0)),
		durationMetric("first_response", "First response", durationOf(h.response, 0)),
		durationMetric("resolution", "Resolution time", durationOf(h.closed, 3)),
		rateMetric("csat", "CSAT", h.ratings.count(1), h.ratings.count(0)),
		rateMetric("first_contact_resolution", "First contact resolution", h.closed.count(2), h.closed.count(0)),
	}, nil
}

/*
headlines are the single rows most reports start from.
*/
type headlines struct {
	started, closed, response, ratings values
}

/*
headline reads the range's started, closed, first response and rating
figures.
*/
func (r reader) headline(ctx context.Context) (headlines, error) {
	var h headlines

	steps := []struct {
		read groupRead
		into *values
		n    int
	}{
		{r.startedFigures, &h.started, 6},
		{r.closedFigures, &h.closed, 7},
		{r.firstResponse, &h.response, 4},
		{r.ratingFigures, &h.ratings, 3},
	}
	for _, s := range steps {
		rows, err := s.read(ctx, "")
		if err != nil {
			return h, err
		}

		*s.into = only(rows, s.n)
	}

	return h, nil
}

/*
Charts returns the charts.
*/
func (o Overview) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	return sequential(ctx, tx, q, o.ChartParts())
}

/*
ChartParts returns the charts as parts that can be read at once.
*/
func (Overview) ChartParts() []ChartPart {
	one := func(read func(r reader, ctx context.Context) (Chart, error)) ChartPart {
		return func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
			c, err := read(reader{tx: tx, q: q}, ctx)

			return []Chart{c}, err
		}
	}

	return []ChartPart{
		one(func(r reader, ctx context.Context) (Chart, error) { return r.volumeChart(ctx) }),
		one(func(r reader, ctx context.Context) (Chart, error) { return r.backlogChart(ctx) }),
		one(func(r reader, ctx context.Context) (Chart, error) { return r.heatmap(ctx) }),
		one(func(r reader, ctx context.Context) (Chart, error) {
			n, err := loadNames(ctx, r.tx, r.q.Membership.Workspace.ID)
			if err != nil {
				return Chart{}, err
			}

			return breakdown{
				key: "channels", title: "Conversations by channel", group: groupChannel, drill: "new_conversations",
				columns: []Column{{Key: "count", Label: "Conversations", Unit: UnitCount}},
				read:    r.startedFigures, pick: func(v values) []*float64 { return []*float64{v.at(0)} },
			}.table(ctx, n)
		}),
	}
}

/*
volumeChart is new against closed conversations per day.
*/
func (r reader) volumeChart(ctx context.Context) (Chart, error) {
	started, err := r.startedFigures(ctx, groupDay)
	if err != nil {
		return Chart{}, err
	}

	closed, err := r.closedFigures(ctx, groupDay)
	if err != nil {
		return Chart{}, err
	}

	return Chart{
		Key: "volume", Title: "New and closed conversations", Kind: ChartBar, Unit: UnitCount, Axis: AxisDay,
		Series: []Series{
			daily(r.q, Series{Key: "new", Label: "New", Drill: "new_conversations"}, started, at(0)),
			daily(r.q, Series{Key: "closed", Label: "Closed", Drill: "closed"}, closed, at(0)),
		},
	}, nil
}

/*
heatmap counts new conversations by weekday and hour, one series per
weekday from Monday, one point per hour.
*/
func (r reader) heatmap(ctx context.Context) (Chart, error) {
	rows, err := r.read(ctx, figures{from: fromFacts, dated: "f.started_at", cols: "count(*)::float8"}, groupHour, 1)
	if err != nil {
		return Chart{}, err
	}

	byKey := make(map[string]*float64, len(rows))
	for _, v := range rows {
		byKey[v.key] = v.at(0)
	}

	out := Chart{
		Key: "busiest_hours", Title: "Busiest hours", Kind: ChartHeatmap, Unit: UnitCount, Axis: AxisHour,
		Drill: "new_conversations",
	}
	for i, day := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		s := Series{Key: string(rune('1' + i)), Label: day}
		for hour := range 24 {
			key := s.Key + "-" + twoDigits(hour)
			s.Points = append(s.Points, Point{X: twoDigits(hour), Y: byKey[key]})
		}

		out.Series = append(out.Series, s)
	}

	return out, nil
}

/*
twoDigits writes an hour as two digits.
*/
func twoDigits(n int) string {
	return fmt.Sprintf("%02d", n)
}

/*
backlogChart is the backlog counted at the end of each day.
*/
func (r reader) backlogChart(ctx context.Context) (Chart, error) {
	q := r.q

	rows, err := models.New(r.tx).ReportBacklogDays(ctx, models.ReportBacklogDaysParams{
		WorkspaceID: q.Membership.Workspace.ID, FromDay: q.Filters.From, ToDay: q.Filters.To,
		Inboxes: orNone(q.Filters.Inboxes), Teams: orNone(q.Filters.Teams),
	})
	if err != nil {
		return Chart{}, err
	}

	asValues := make([]values, len(rows))
	for i, b := range rows {
		asValues[i] = values{key: b.Day, nums: []*float64{num(b.Open), num(b.Unassigned), num(b.OverWeek)}}
	}

	return Chart{
		Key: "backlog", Title: "Backlog at the end of each day", Kind: ChartLine, Unit: UnitCount, Axis: AxisDay,
		Series: []Series{
			daily(q, Series{Key: "open", Label: "Open"}, asValues, at(0)),
			daily(q, Series{Key: "unassigned", Label: "Unassigned"}, asValues, at(1)),
			daily(q, Series{Key: "over_week", Label: "Waiting over a week"}, asValues, at(2)),
		},
	}, nil
}
