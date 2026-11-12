/*
|--------------------------------------------------------------------------
| Ticket Report Routes
|--------------------------------------------------------------------------
|
| The ticket reports: time in state, tickets by type and by a field's
| values, service level attainment and tracker impact, each over a
| range of days. Members need tickets:read.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
ticketReportRoutes declares the ticket reports.
*/
func ticketReportRoutes(workspace *routing.Router, a *app.App) {
	rc := &v1.TicketReportController{Reports: a.Actions.TicketReports, IDs: a.IDs}
	base := "/reports/tickets"

	routing.Get(workspace, base+"/time-in-state", "reports.tickets.time-in-state.index", rc.TimeInState).
		Ability("tickets:read")
	routing.Get(workspace, base+"/by-type", "reports.tickets.by-type.index", rc.ByType).Ability("tickets:read")
	routing.Get(workspace, base+"/by-field", "reports.tickets.by-field.index", rc.ByField).Ability("tickets:read")
	routing.Get(workspace, base+"/sla", "reports.tickets.sla.index", rc.SLA).Ability("tickets:read")
	routing.Get(workspace, base+"/trackers", "reports.tickets.trackers.index", rc.Trackers).Ability("tickets:read")
}
