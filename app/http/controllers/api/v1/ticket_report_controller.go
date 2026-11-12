/*
|--------------------------------------------------------------------------
| Ticket Report Controller
|--------------------------------------------------------------------------
|
| GET /reports/tickets/time-in-state, /by-type, /by-field, /sla and
| /trackers answer the ticket reports for a range of days: time spent
| in each state per type, tickets per type and per value of a field,
| service level attainment per policy and target, and the trackers
| ranked by what they reach. Members need tickets:read.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ticketreports"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketReportController serves the ticket reports.
*/
type TicketReportController struct {
	Reports *ticketreports.Reports
	IDs     *ids.Registry
}

/*
rangeOf reads a request's days.
*/
func rangeOf(in requests.TicketReportRequest) ticketreports.Range {
	return ticketreports.Range{From: in.From, To: in.To}
}

/*
TimeInState answers the time in state report.
*/
func (c *TicketReportController) TimeInState(
	ctx context.Context, in *requests.TicketReportRequest,
) (*TicketStateTimesOutput, error) {
	rows, err := c.Reports.TimeInState(ctx, membership(ctx), rangeOf(*in))
	if err != nil {
		return nil, err
	}

	return &TicketStateTimesOutput{Body: fhttp.NewResource(resources.NewTicketStateTimes(rows))}, nil
}

/*
ByType answers the tickets by type report.
*/
func (c *TicketReportController) ByType(
	ctx context.Context, in *requests.TicketReportRequest,
) (*TicketTypeCountsOutput, error) {
	rows, err := c.Reports.ByType(ctx, membership(ctx), rangeOf(*in))
	if err != nil {
		return nil, err
	}

	return &TicketTypeCountsOutput{Body: fhttp.NewResource(resources.NewTicketTypeCounts(rows))}, nil
}

/*
ByField answers the tickets by field report.
*/
func (c *TicketReportController) ByField(
	ctx context.Context, in *requests.TicketFieldReportRequest,
) (*TicketFieldReportOutput, error) {
	typeID, err := c.IDs.Decode(string(models.PrefixTicketType), in.TypeID)
	if err != nil {
		return nil, ticketreports.ErrTypeNotFound
	}

	r, err := c.Reports.ByField(ctx, membership(ctx), ticketreports.FieldInput{
		Range: rangeOf(in.TicketReportRequest), TypeID: typeID, Field: in.Field,
	})
	if err != nil {
		return nil, err
	}

	return &TicketFieldReportOutput{Body: fhttp.NewResource(resources.NewTicketFieldReport(r))}, nil
}

/*
SLA answers the service level report.
*/
func (c *TicketReportController) SLA(
	ctx context.Context, in *requests.TicketReportRequest,
) (*TicketSLAReportOutput, error) {
	rows, err := c.Reports.SLA(ctx, membership(ctx), rangeOf(*in))
	if err != nil {
		return nil, err
	}

	return &TicketSLAReportOutput{Body: fhttp.NewResource(resources.NewTicketSLAAttainments(rows))}, nil
}

/*
Trackers answers the tracker impact ranking.
*/
func (c *TicketReportController) Trackers(
	ctx context.Context, in *requests.TicketReportRequest,
) (*TicketTrackerReportOutput, error) {
	rows, err := c.Reports.Trackers(ctx, membership(ctx), rangeOf(*in))
	if err != nil {
		return nil, err
	}

	return &TicketTrackerReportOutput{Body: fhttp.NewResource(resources.NewTicketTrackerImpacts(rows))}, nil
}
