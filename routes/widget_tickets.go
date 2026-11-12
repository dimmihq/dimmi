/*
|--------------------------------------------------------------------------
| Widget Ticket Routes
|--------------------------------------------------------------------------
|
| The messenger's requests and status card. The types a visitor may
| open and the active incidents read without a session too, under the
| app's own address, so Home shows them before the visitor writes.
| Opening a request, and reading one's own, needs the session; replies
| go through the conversation's message routes.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/http/useragent"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/widget"
)

/*
widgetAppTicketRoutes declares what any visitor of the app reads.
*/
func widgetAppTicketRoutes(public *routing.Router, a *app.App) {
	if a.Actions.Tickets == nil {
		return
	}

	types := widgetTickets(a)
	incidents := &widget.IncidentController{Tickets: a.Actions.Tickets}

	routing.Get(public, "/apps/{app}/ticket-types", "widget.apps.ticket-types.index", types.Types)
	routing.Get(public, "/apps/{app}/incidents", "widget.apps.incidents.index", incidents.Index)
}

/*
widgetTicketRoutes declares the visitor's requests.
*/
func widgetTicketRoutes(visitor *routing.Router, a *app.App) {
	if a.Actions.Tickets == nil {
		return
	}

	c := widgetTickets(a)
	incidents := &widget.IncidentController{Tickets: a.Actions.Tickets}

	routing.Get(visitor, "/ticket-types", "widget.ticket-types.index", c.Types)
	routing.Get(visitor, "/tickets", "widget.tickets.index", c.Index)
	routing.Post(visitor, "/tickets", "widget.tickets.store", c.Store).Middleware("idempotent")
	routing.Get(visitor, "/tickets/{ticket}", "widget.tickets.show", c.Show)
	routing.Get(visitor, "/incidents", "widget.incidents.index", incidents.Index)
}

/*
widgetTickets returns the controller of the visitor's requests.
*/
func widgetTickets(a *app.App) *widget.TicketController {
	return &widget.TicketController{
		Tickets: a.Actions.Tickets, Post: a.Actions.Widget.PostMessage, IDs: a.IDs, Clock: a.Clock,
		Agents: useragent.New(),
	}
}
