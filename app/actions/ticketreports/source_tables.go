/*
|--------------------------------------------------------------------------
| Tickets And SLAs Report Tables
|--------------------------------------------------------------------------
|
| The report's breakdowns: tickets per type as bars, and tables of the
| time spent in each state per type, attainment per policy and target,
| and the trackers ranked by the revenue they reach.
|
*/

package ticketreports

import (
	"context"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
typeChart tables tickets created and resolved per type in the range,
and open now.
*/
func typeChart(ctx context.Context, mq *models.Queries, q reports.Query) (reports.Chart, error) {
	types, err := byType(ctx, mq, q)
	if err != nil {
		return reports.Chart{}, err
	}

	out := reports.Chart{
		Key: "tickets_by_type", Title: "Tickets by type", Kind: reports.ChartTable, Unit: reports.UnitCount,
		Columns: []reports.Column{
			{Key: "created", Label: "Created", Unit: reports.UnitCount},
			{Key: "resolved", Label: "Resolved", Unit: reports.UnitCount},
			{Key: "open", Label: "Open now", Unit: reports.UnitCount},
		},
	}

	for _, t := range types {
		out.Rows = append(out.Rows, reports.Row{
			Key: models.PublicID(models.PrefixTicketType, t.Type.ID), Label: t.Type.Name,
			Values: []*float64{count(t.Created), count(t.Resolved), count(t.Open)},
		})
	}

	return out, nil
}

/*
stateTable tables the time spent in each state per type.
*/
func stateTable(ctx context.Context, mq *models.Queries, q reports.Query) (reports.Chart, error) {
	rows, err := timeInState(ctx, mq, q)
	if err != nil {
		return reports.Chart{}, err
	}

	out := reports.Chart{
		Key: "time_in_state", Title: "Time in state", Kind: reports.ChartTable, Unit: reports.UnitSeconds,
		Columns: []reports.Column{
			{Key: "tickets", Label: "Tickets", Unit: reports.UnitCount},
			{Key: "median", Label: "Median", Unit: reports.UnitSeconds},
			{Key: "p90", Label: "90th percentile", Unit: reports.UnitSeconds},
		},
	}

	for _, r := range rows {
		out.Rows = append(out.Rows, reports.Row{
			Key: r.TypeID.String() + ":" + r.StateID.String(), Label: r.TypeName + " · " + r.State,
			Values: []*float64{count(int64(r.Tickets)), &r.Median, &r.P90},
		})
	}

	return out, nil
}

/*
slaTable tables attainment per policy and target.
*/
func slaTable(ctx context.Context, mq *models.Queries, q reports.Query) (reports.Chart, error) {
	rows, err := attainment(ctx, mq, q)
	if err != nil {
		return reports.Chart{}, err
	}

	out := reports.Chart{
		Key: "sla_attainment", Title: "Service levels by policy", Kind: reports.ChartTable,
		Unit: reports.UnitPercent,
		Columns: []reports.Column{
			{Key: "met", Label: "Met", Unit: reports.UnitCount},
			{Key: "breached", Label: "Breached", Unit: reports.UnitCount},
			{Key: "hit_rate", Label: "Kept", Unit: reports.UnitPercent},
			{Key: "running", Label: "Running now", Unit: reports.UnitCount},
		},
	}

	for _, r := range rows {
		out.Rows = append(out.Rows, reports.Row{
			Key: r.PolicyID.String() + ":" + r.Target, Label: r.PolicyName + " · " + targetLabel(r.Target),
			Values: []*float64{count(r.Met), count(r.Breached), r.Percent, count(r.Running)},
		})
	}

	return out, nil
}

/*
targetLabel names a service level target for people.
*/
func targetLabel(target string) string {
	switch target {
	case "first_reply":
		return "First reply"
	case "next_reply":
		return "Next reply"
	case "follow_up":
		return "Follow-up"
	case "resolution":
		return "Resolution"
	}

	return target
}

/*
trackerTable tables the trackers ranked by what they reach.
*/
func trackerTable(ctx context.Context, mq *models.Queries, q reports.Query) (reports.Chart, error) {
	rows, err := trackers(ctx, mq, q)
	if err != nil {
		return reports.Chart{}, err
	}

	out := reports.Chart{
		Key: "tracker_impact", Title: "Tracker impact", Kind: reports.ChartTable, Unit: reports.UnitCount,
		Columns: []reports.Column{
			{Key: "customers", Label: "Customers", Unit: reports.UnitCount},
			{Key: "companies", Label: "Companies", Unit: reports.UnitCount},
			{Key: "revenue", Label: "Monthly revenue", Unit: reports.UnitCount},
		},
	}

	for _, r := range rows {
		revenue := r.Revenue
		out.Rows = append(out.Rows, reports.Row{
			Key: models.PublicID(models.PrefixTicket, r.ID), Label: tickets.Reference(r.Number) + " " + r.Title,
			Values: []*float64{count(r.Customers), count(r.Companies), &revenue},
		})
	}

	return out, nil
}
