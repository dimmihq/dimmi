/*
|--------------------------------------------------------------------------
| Tickets And SLAs Report Fields
|--------------------------------------------------------------------------
|
| The report splits each live type's tickets created in the range by the
| values of its choice fields, one bar chart per type and field, such as
| refund reasons or bug severity. Fields free text would scatter, such
| as order numbers, are left to the by-field endpoint.
|
*/

package ticketreports

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxFieldCharts is how many field charts the report shows at most.
*/
const maxFieldCharts = 8

/*
fieldCharts charts each choice field of each live type by its values.
*/
func fieldCharts(ctx context.Context, mq *models.Queries, q reports.Query) ([]reports.Chart, error) {
	ws := q.Membership.Workspace.ID

	types, err := mq.ListTicketTypes(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list ticket types: %w", err)
	}

	fields, err := mq.ListTicketFields(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list ticket fields: %w", err)
	}

	var out []reports.Chart

	for _, ty := range types {
		for _, f := range fields {
			if len(out) == maxFieldCharts {
				return out, nil
			}

			if !charted(ty, f) {
				continue
			}

			c, err := fieldChart(ctx, mq, q, FieldInput{TypeID: ty.ID, Field: f.Key})
			if err != nil {
				return nil, err
			}

			out = append(out, c)
		}
	}

	return out, nil
}

/*
charted reports whether the field of the type gets a chart: a live
choice field of a live type.
*/
func charted(ty models.TicketType, f models.TicketField) bool {
	choice := slices.Contains([]string{"single_choice", "multiple_choice", "boolean"}, f.DataType)

	return choice && f.TypeID == ty.ID && f.ArchivedAt == nil && ty.ArchivedAt == nil
}

/*
fieldChart charts one field's values as bars.
*/
func fieldChart(ctx context.Context, mq *models.Queries, q reports.Query, in FieldInput) (reports.Chart, error) {
	r, err := byField(ctx, mq, q, in)
	if err != nil {
		return reports.Chart{}, err
	}

	s := reports.Series{Key: "tickets", Label: "Tickets"}
	for _, v := range r.Values {
		label := "None"
		if v.Value != nil {
			label = *v.Value
		}

		s.Points = append(s.Points, reports.Point{X: label, Y: count(v.Tickets)})
	}

	return reports.Chart{
		Key:   "by_field:" + models.PublicID(models.PrefixTicketType, r.Type.ID) + ":" + r.Field.Key,
		Title: r.Type.Name + " by " + r.Field.Label, Kind: reports.ChartBar, Unit: reports.UnitCount,
		Axis: reports.AxisCategory, Series: []reports.Series{s},
	}, nil
}
