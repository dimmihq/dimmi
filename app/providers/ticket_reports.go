/*
|--------------------------------------------------------------------------
| Ticket Report Wiring
|--------------------------------------------------------------------------
|
| The ticket reports answer their own endpoints, and the tickets and
| SLAs report joins the conversation reports in the registry, so the
| reports area and GET /reports/tickets show it.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/ticketreports"
)

/*
bootTicketReports builds the ticket reports and registers the report.
*/
func bootTicketReports(a *app.App) {
	a.Actions.TicketReports = &ticketreports.Reports{DB: a.DB, Clock: a.Clock}

	if a.Actions.Reports != nil {
		a.Actions.Reports.Register(ticketreports.Source{})
	}
}
