/*
|--------------------------------------------------------------------------
| SLA Routes
|--------------------------------------------------------------------------
|
| Service level policies, which members who change settings manage and
| every member reads, and the clocks they run on tickets and
| conversations, read like the tickets and conversations themselves.
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
slaRoutes declares service level policies and clocks.
*/
func slaRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.SLA == nil {
		return
	}

	sc := &v1.SLAController{
		SLA: a.Actions.SLA, IDs: a.IDs, Tickets: &v1.TicketController{Tickets: a.Actions.Tickets, IDs: a.IDs},
	}
	one := "/sla-policies/{policy}"

	routing.Get(workspace, "/sla-policies", "sla-policies.index", sc.Index).Ability("settings:read")
	routing.Post(workspace, "/sla-policies", "sla-policies.store", sc.Store).Ability("settings:write").
		Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "sla-policies.show", sc.Show).Ability("settings:read")
	routing.Patch(workspace, one, "sla-policies.update", sc.Update).Ability("settings:write")
	routing.Delete(workspace, one, "sla-policies.destroy", sc.Destroy).Ability("settings:write").
		Status(fhttp.StatusNoContent)
	routing.Get(workspace, "/tickets/{ticket}/sla-clocks", "tickets.sla-clocks.index", sc.TicketClocks).
		Ability("tickets:read")
	routing.Get(workspace, "/conversations/{conversation}/sla-clocks", "conversations.sla-clocks.index",
		sc.ConversationClocks).Ability("conversations:read")
}
