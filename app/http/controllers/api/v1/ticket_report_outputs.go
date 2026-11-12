/*
|--------------------------------------------------------------------------
| Ticket Report Outputs
|--------------------------------------------------------------------------
|
| What the ticket report endpoints answer.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
TicketStateTimesOutput is the time in state report.
*/
type TicketStateTimesOutput struct {
	Body fhttp.Resource[[]resources.TicketStateTime]
}

/*
TicketTypeCountsOutput is the tickets by type report.
*/
type TicketTypeCountsOutput struct {
	Body fhttp.Resource[[]resources.TicketTypeCount]
}

/*
TicketFieldReportOutput is the tickets by field report.
*/
type TicketFieldReportOutput struct {
	Body fhttp.Resource[resources.TicketFieldReport]
}

/*
TicketSLAReportOutput is the service level report.
*/
type TicketSLAReportOutput struct {
	Body fhttp.Resource[[]resources.TicketSLAAttainment]
}

/*
TicketTrackerReportOutput is the tracker impact ranking.
*/
type TicketTrackerReportOutput struct {
	Body fhttp.Resource[[]resources.TicketTrackerImpact]
}
