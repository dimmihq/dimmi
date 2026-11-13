/*
|--------------------------------------------------------------------------
| Tracker Routes
|--------------------------------------------------------------------------
|
| The trackers the AI suggests for a conversation, and what a tracker's
| customers report.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
trackerRoutes declares tracker suggestions and summaries.
*/
func trackerRoutes(workspace *routing.Router, a *app.App) {
	tc := &v1.TrackerController{TicketController: v1.TicketController{Tickets: a.Actions.Tickets, IDs: a.IDs}}

	routing.Get(workspace, "/conversations/{conversation}/tracker-suggestions",
		"conversations.tracker-suggestions.index", tc.TrackerSuggestions).Ability("tickets:read")
	routing.Get(workspace, "/tickets/{ticket}/summary", "tickets.summary.show", tc.Summary).Ability("tickets:read")
}
