/*
|--------------------------------------------------------------------------
| Incident Routes
|--------------------------------------------------------------------------
|
| Suggested incidents the spike detector found, which members who work
| tickets accept or dismiss; the trackers the AI suggests for a
| conversation; and what a tracker's customers report.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
incidentRoutes declares suggestions and summaries.
*/
func incidentRoutes(workspace *routing.Router, a *app.App) {
	ic := &v1.IncidentController{TicketController: v1.TicketController{Tickets: a.Actions.Tickets, IDs: a.IDs}}

	routing.Get(workspace, "/incident-suggestions", "incident-suggestions.index", ic.Suggestions).
		Ability("tickets:read")
	routing.Patch(workspace, "/incident-suggestions/{suggestion}", "incident-suggestions.update", ic.Decide).
		Ability("tickets:write")
	routing.Get(workspace, "/conversations/{conversation}/tracker-suggestions",
		"conversations.tracker-suggestions.index", ic.TrackerSuggestions).Ability("tickets:read")
	routing.Get(workspace, "/tickets/{ticket}/summary", "tickets.summary.show", ic.Summary).Ability("tickets:read")
}
