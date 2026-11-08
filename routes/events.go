/*
|--------------------------------------------------------------------------
| Event Routes
|--------------------------------------------------------------------------
|
| The API of events. Recording them takes the contacts:write ability,
| reading names and summaries contacts:read, and changing a name
| attributes:write.
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
eventRoutes declares the events API.
*/
func eventRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Events == nil {
		return
	}

	ec := &v1.EventController{Events: a.Actions.Events, IDs: a.IDs, Clock: a.Clock}

	routing.Post(workspace, "/contact-events", "contact-events.store", ec.Track).
		Ability("contacts:write").Status(fhttp.StatusOK)
	routing.Get(workspace, "/event-definitions", "event-definitions.index", ec.Index).Ability("contacts:read")
	routing.Patch(workspace, "/event-definitions/{event}", "event-definitions.update", ec.Update).
		Ability("attributes:write")
	routing.Get(workspace, "/contacts/{contact}/event-summaries", "contacts.event-summaries.index", ec.Summaries).
		Ability("contacts:read")
}
