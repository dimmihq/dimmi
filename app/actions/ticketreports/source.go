/*
|--------------------------------------------------------------------------
| Tickets And SLAs Report
|--------------------------------------------------------------------------
|
| The ticket numbers as a report beside the conversation reports: the
| headline counts and the share of service level targets kept, tickets
| created and resolved per day, targets met and breached per day,
| tickets per type, and tables of time in state, attainment per policy
| and the trackers that reach the most revenue. The registry runs it in
| the workspace's transaction for the range asked.
|
*/

package ticketreports

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Source is the tickets and SLAs report.
*/
type Source struct{}

/*
Describe says what the report is.
*/
func (Source) Describe() reports.Description {
	return reports.Description{
		Key: "tickets", Title: "Tickets and SLAs", Order: 90, Permission: "tickets:read",
		Summary: "Tickets by type and field, time in each state, service levels kept and trackers' reach.",
	}
}

/*
Metrics returns the ticket counts and the service levels kept.
*/
func (Source) Metrics(ctx context.Context, tx *database.Tx, q reports.Query) ([]reports.Metric, error) {
	mq := models.New(tx)

	types, err := byType(ctx, mq, q)
	if err != nil {
		return nil, err
	}

	var created, resolved, open int64
	for _, t := range types {
		created, resolved, open = created+t.Created, resolved+t.Resolved, open+t.Open
	}

	slas, err := attainment(ctx, mq, q)
	if err != nil {
		return nil, err
	}

	var met, breached int64
	for _, a := range slas {
		met, breached = met+a.Met, breached+a.Breached
	}

	return []reports.Metric{
		{Key: "tickets_created", Label: "Tickets created", Unit: reports.UnitCount, Value: count(created)},
		{Key: "tickets_resolved", Label: "Tickets resolved", Unit: reports.UnitCount, Value: count(resolved)},
		{Key: "tickets_open", Label: "Open tickets", Unit: reports.UnitCount, Value: count(open)},
		{
			Key: "sla_hit_rate", Label: "Service levels kept", Unit: reports.UnitPercent,
			Value: percent(met, met+breached), Sample: met + breached,
		},
		{Key: "sla_breached", Label: "Targets breached", Unit: reports.UnitCount, Value: count(breached)},
	}, nil
}

/*
count returns a count as a number.
*/
func count(n int64) *float64 {
	v := float64(n)

	return &v
}

/*
Charts returns the report's charts.
*/
func (Source) Charts(ctx context.Context, tx *database.Tx, q reports.Query) ([]reports.Chart, error) {
	mq := models.New(tx)
	out := make([]reports.Chart, 0, 6+maxFieldCharts)

	for _, chart := range []func(context.Context, *models.Queries, reports.Query) (reports.Chart, error){
		dailyTickets, dailySLA, typeChart, stateTable, slaTable, trackerTable,
	} {
		c, err := chart(ctx, mq, q)
		if err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	fields, err := fieldCharts(ctx, mq, q)
	if err != nil {
		return nil, err
	}

	return append(out, fields...), nil
}

/*
days returns every day of the range.
*/
func days(q reports.Query) []string {
	out := make([]string, 0, q.Days)
	for d := q.Start; d.Before(q.End); d = d.AddDate(0, 0, 1) {
		out = append(out, d.In(q.Location).Format(time.DateOnly))
	}

	return out
}

/*
daySeries returns a series with a point for every day of the range,
zero on days without a value.
*/
func daySeries(key, label string, all []string, values map[string]int64) reports.Series {
	s := reports.Series{Key: key, Label: label, Points: make([]reports.Point, 0, len(all))}
	for _, d := range all {
		s.Points = append(s.Points, reports.Point{X: d, Y: count(values[d])})
	}

	return s
}

/*
dailyTickets charts tickets created and resolved each day.
*/
func dailyTickets(ctx context.Context, mq *models.Queries, q reports.Query) (reports.Chart, error) {
	a := access(q)

	rows, err := mq.CountReportTicketsDaily(ctx, models.CountReportTicketsDailyParams{
		Zone: q.Location.String(), WorkspaceID: q.Membership.Workspace.ID, RangeFrom: q.Start, RangeTo: q.End,
		AccessMode: a.Mode, AccessViewer: a.Viewer, AccessTeams: a.Teams, AccessUnassigned: a.Unassigned,
		AccessHiddenInboxes: a.HiddenInboxes,
	})
	if err != nil {
		return reports.Chart{}, fmt.Errorf("count tickets per day: %w", err)
	}

	created, resolved := map[string]int64{}, map[string]int64{}
	for _, r := range rows {
		created[r.Day], resolved[r.Day] = r.Created, r.Resolved
	}

	all := days(q)

	return reports.Chart{
		Key: "tickets_per_day", Title: "Tickets created and resolved", Kind: reports.ChartLine,
		Unit: reports.UnitCount, Axis: reports.AxisDay,
		Series: []reports.Series{
			daySeries("created", "Created", all, created), daySeries("resolved", "Resolved", all, resolved),
		},
	}, nil
}

/*
dailySLA charts the targets met and breached each day.
*/
func dailySLA(ctx context.Context, mq *models.Queries, q reports.Query) (reports.Chart, error) {
	a := access(q)

	rows, err := mq.CountReportSLADaily(ctx, models.CountReportSLADailyParams{
		Zone: q.Location.String(), WorkspaceID: q.Membership.Workspace.ID, RangeFrom: q.Start, RangeTo: q.End,
		AccessMode: a.Mode, AccessViewer: a.Viewer, AccessTeams: a.Teams, AccessUnassigned: a.Unassigned,
		AccessHiddenInboxes: a.HiddenInboxes,
	})
	if err != nil {
		return reports.Chart{}, fmt.Errorf("count sla clocks per day: %w", err)
	}

	met, breached := map[string]int64{}, map[string]int64{}
	for _, r := range rows {
		met[r.Day], breached[r.Day] = r.Met, r.Breached
	}

	all := days(q)

	return reports.Chart{
		Key: "sla_per_day", Title: "Service level targets", Kind: reports.ChartStacked, Unit: reports.UnitCount,
		Axis:   reports.AxisDay,
		Series: []reports.Series{daySeries("met", "Met", all, met), daySeries("breached", "Breached", all, breached)},
	}, nil
}
