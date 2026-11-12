/*
|--------------------------------------------------------------------------
| Ticket Report Resources
|--------------------------------------------------------------------------
|
| What the ticket reports answer. Durations are seconds, percentages
| run from 0 to 100 and are null with nothing to count, and revenue is
| the monthly revenue of the companies a tracker reaches, read from
| their monthly_spend or mrr attribute.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/ticketreports"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketStateTime is the time tickets of one type spent in one state.
*/
type TicketStateTime struct {
	TypeID        string  `json:"type_id"`
	TypeName      string  `json:"type_name"`
	StateID       string  `json:"state_id"`
	StateName     string  `json:"state_name"`
	Group         string  `json:"group"`
	Tickets       int     `json:"tickets"`
	MeanSeconds   float64 `json:"mean_seconds"`
	MedianSeconds float64 `json:"median_seconds"`
	P90Seconds    float64 `json:"p90_seconds"`
}

/*
NewTicketStateTimes shapes the time in state rows.
*/
func NewTicketStateTimes(rows []ticketreports.StateTime) []TicketStateTime {
	out := make([]TicketStateTime, 0, len(rows))
	for _, r := range rows {
		out = append(out, TicketStateTime{
			TypeID: models.PublicID(models.PrefixTicketType, r.TypeID), TypeName: r.TypeName,
			StateID: models.PublicID(models.PrefixTicketState, r.StateID), StateName: r.State, Group: r.Group,
			Tickets: r.Tickets, MeanSeconds: r.Mean, MedianSeconds: r.Median, P90Seconds: r.P90,
		})
	}

	return out
}

/*
TicketTypeCount is one type's tickets.
*/
type TicketTypeCount struct {
	TypeID   string `json:"type_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Created  int64  `json:"created"`
	Resolved int64  `json:"resolved"`
	Open     int64  `json:"open"`
}

/*
NewTicketTypeCounts shapes the per type counts.
*/
func NewTicketTypeCounts(rows []ticketreports.TypeCount) []TicketTypeCount {
	out := make([]TicketTypeCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, TicketTypeCount{
			TypeID: models.PublicID(models.PrefixTicketType, r.Type.ID), Name: r.Type.Name, Kind: r.Type.Kind,
			Created: r.Created, Resolved: r.Resolved, Open: r.Open,
		})
	}

	return out
}

/*
TicketFieldReport is a field of a type and how its values split.
*/
type TicketFieldReport struct {
	TypeID   string             `json:"type_id"`
	Field    string             `json:"field"`
	Label    string             `json:"label"`
	DataType string             `json:"data_type"`
	Values   []TicketFieldCount `json:"values"`
}

/*
TicketFieldCount is how many tickets hold one value, null for none.
*/
type TicketFieldCount struct {
	Value    *string `json:"value"`
	Tickets  int64   `json:"tickets"`
	Resolved int64   `json:"resolved"`
}

/*
NewTicketFieldReport shapes a field's counts.
*/
func NewTicketFieldReport(r ticketreports.FieldReport) TicketFieldReport {
	out := TicketFieldReport{
		TypeID: models.PublicID(models.PrefixTicketType, r.Type.ID), Field: r.Field.Key, Label: r.Field.Label,
		DataType: r.Field.DataType, Values: make([]TicketFieldCount, 0, len(r.Values)),
	}

	for _, v := range r.Values {
		out.Values = append(out.Values, TicketFieldCount{Value: v.Value, Tickets: v.Tickets, Resolved: v.Resolved})
	}

	return out
}
