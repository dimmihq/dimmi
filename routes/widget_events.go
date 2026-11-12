/*
|--------------------------------------------------------------------------
| Widget Event Routes
|--------------------------------------------------------------------------
|
| What the visitor did on the page, sent by their messenger in batches.
| The session names the person; only names opened to the messenger are
| recorded, unless the workspace lets it send any.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/controllers/widget"
)

/*
widgetEventRoutes declares the visitor's events.
*/
func widgetEventRoutes(visitor *routing.Router, a *app.App) {
	if a.Actions.Events == nil {
		return
	}

	c := &widget.EventController{Track: &actions.TrackWidgetEvents{
		Events: a.Actions.Events, Limits: &actions.VisitorLimits{Limiter: a.Limiter},
	}}
	routing.Post(visitor, "/events", "widget.events.store", c.Store).Status(fhttp.StatusOK)
}
