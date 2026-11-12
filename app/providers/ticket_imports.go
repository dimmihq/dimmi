/*
|--------------------------------------------------------------------------
| Ticket Import Wiring
|--------------------------------------------------------------------------
|
| Imports of tickets from Intercom and Zendesk write through the tickets
| action, so they are built after it; undoing one runs in a job.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/ticketimports"
)

/*
bootTicketImports builds the ticket importer and registers its undo.
*/
func bootTicketImports(a *app.App) error {
	importer := &ticketimports.Importer{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue, Tickets: a.Actions.Tickets,
	}
	a.Actions.TicketImports = importer

	return queue.Register(a.Queue, &ticketimports.RollbackWorker{Importer: importer})
}
