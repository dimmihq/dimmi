/*
|--------------------------------------------------------------------------
| Ticket Routes
|--------------------------------------------------------------------------
|
| The tickets API: types, fields and states, which members who change
| settings manage; tickets with their history, notes and links; a
| conversation turned into a ticket; trackers' conversations and the
| messages that reach all of them; and incidents' public logs. Members
| need tickets:read to read and tickets:write to change.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
ticketRoutes declares tickets and what hangs off them.
*/
func ticketRoutes(workspace *routing.Router, a *app.App) {
	base := v1.TicketController{Tickets: a.Actions.Tickets, IDs: a.IDs}
	tc := &base
	one := "/tickets/{ticket}"

	routing.Get(workspace, "/tickets", "tickets.index", tc.Index).Ability("tickets:read")
	routing.Post(workspace, "/tickets", "tickets.store", tc.Store).Ability("tickets:write").
		Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "tickets.show", tc.Show).Ability("tickets:read")
	routing.Patch(workspace, one, "tickets.update", tc.Update).Ability("tickets:write")
	routing.Delete(workspace, one, "tickets.destroy", tc.Destroy).Ability("tickets:write").
		Status(fhttp.StatusNoContent)
	routing.Post(workspace, one+"/restore", "tickets.restore.store", tc.Restore).Ability("tickets:write").
		Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/escalations", "tickets.escalations.store", tc.Escalate).
		Ability("tickets:write").Status(fhttp.StatusCreated)
	routing.Post(workspace, "/conversations/{conversation}/ticket", "conversations.ticket.store", tc.Convert).
		Ability("tickets:write").Status(fhttp.StatusCreated)

	ticketActivityRoutes(workspace, base)
	ticketSetupRoutes(workspace, base)
	ticketReportRoutes(workspace, a)
}

/*
ticketActivityRoutes declares history, notes, links, trackers' reach
and incidents' public logs.
*/
func ticketActivityRoutes(workspace *routing.Router, base v1.TicketController) {
	ac := &v1.TicketActivityController{TicketController: base}
	one := "/tickets/{ticket}"
	conv := "/conversations/{conversation}"

	routing.Get(workspace, one+"/events", "tickets.events.index", ac.Events).Ability("tickets:read")
	routing.Get(workspace, "/ticket-events", "ticket-events.index", ac.Export).Ability("tickets:read")
	routing.Post(workspace, one+"/notes", "tickets.notes.store", ac.Note).Ability("tickets:write").
		Status(fhttp.StatusCreated)
	routing.Get(workspace, one+"/links", "tickets.links.index", ac.Links).Ability("tickets:read")
	routing.Post(workspace, one+"/links", "tickets.links.store", ac.Link).Ability("tickets:write").
		Status(fhttp.StatusOK)
	routing.Delete(workspace, "/ticket-links/{link}", "ticket-links.destroy", ac.Unlink).Ability("tickets:write").
		Status(fhttp.StatusNoContent)
	routing.Get(workspace, one+"/conversations", "tickets.conversations.index", ac.Conversations).
		Ability("tickets:read")
	routing.Post(workspace, one+"/updates", "tickets.updates.store", ac.SendUpdate).Ability("tickets:write").
		Status(fhttp.StatusOK)
	routing.Get(workspace, one+"/updates", "tickets.updates.index", ac.Updates).Ability("tickets:read")
	routing.Get(workspace, one+"/public-updates", "tickets.public-updates.index", ac.PublicUpdates).
		Ability("tickets:read")
	routing.Post(workspace, one+"/public-updates", "tickets.public-updates.store", ac.PostPublicUpdate).
		Ability("tickets:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, conv+"/tickets", "conversations.tickets.index", ac.ConversationTickets).
		Ability("tickets:read")
	routing.Get(workspace, conv+"/ticket-suggestions", "conversations.ticket-suggestions.index", ac.Suggestions).
		Ability("tickets:read")
}

/*
ticketSetupRoutes declares types, fields and states.
*/
func ticketSetupRoutes(workspace *routing.Router, base v1.TicketController) {
	sc := &v1.TicketSetupController{TicketController: base}
	ty, field, state := "/ticket-types/{type}", "/ticket-fields/{field}", "/ticket-states/{state}"

	routing.Get(workspace, "/ticket-types", "ticket-types.index", sc.Types).Ability("tickets:read")
	routing.Post(workspace, "/ticket-types", "ticket-types.store", sc.StoreType).Ability("settings:write").
		Status(fhttp.StatusCreated)
	routing.Get(workspace, ty, "ticket-types.show", sc.ShowType).Ability("tickets:read")
	routing.Patch(workspace, ty, "ticket-types.update", sc.UpdateType).Ability("settings:write")
	routing.Delete(workspace, ty, "ticket-types.destroy", sc.DestroyType).Ability("settings:write").
		Status(fhttp.StatusNoContent)

	routing.Get(workspace, ty+"/fields", "ticket-types.fields.index", sc.Fields).Ability("tickets:read")
	routing.Post(workspace, ty+"/fields", "ticket-types.fields.store", sc.StoreField).Ability("settings:write").
		Status(fhttp.StatusCreated)
	routing.Patch(workspace, ty+"/fields", "ticket-types.fields.update-many", sc.ReorderFields).
		Ability("settings:write")
	routing.Patch(workspace, field, "ticket-fields.update", sc.UpdateField).Ability("settings:write")
	routing.Delete(workspace, field, "ticket-fields.destroy", sc.DestroyField).Ability("settings:write").
		Status(fhttp.StatusNoContent)

	routing.Get(workspace, "/ticket-states", "ticket-states.index", sc.States).Ability("tickets:read")
	routing.Post(workspace, "/ticket-states", "ticket-states.store", sc.StoreState).Ability("settings:write").
		Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/ticket-states", "ticket-states.update-many", sc.ReorderStates).
		Ability("settings:write")
	routing.Patch(workspace, state, "ticket-states.update", sc.UpdateState).Ability("settings:write")
	routing.Delete(workspace, state, "ticket-states.destroy", sc.DestroyState).Ability("settings:write").
		Status(fhttp.StatusNoContent)
}
