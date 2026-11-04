/*
|--------------------------------------------------------------------------
| Widget Settings Routes
|--------------------------------------------------------------------------
|
| The dashboard's part of the messenger, inside the workspace a request names.
| Reading takes settings:read and writing settings:write, and each action
| checks the widget permissions on top, so a token can never do more than the
| member who minted it.
|
*/

package routes

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
widgetSettingsRoutes declares the routes on a workspace's widgets.
*/
func widgetSettingsRoutes(r *routing.Router, a *app.App) {
	r.Middleware("csrf", "session", "auth", "workspace", "access.policy")

	acts := a.Actions.Widget
	c := &v1.WidgetSettingsController{
		Read: acts.ShowSettings, Create: acts.Create, Change: acts.Update, Remove: acts.Delete, IDs: a.IDs,
		Base: strings.TrimRight(a.Application.Config.App.URL, "/"),
	}
	one := "/widget-settings/{widget}"

	routing.Get(r, "/widget-settings", "widget-settings.index", c.Index).Ability("settings:read")
	routing.Post(r, "/widget-settings", "widget-settings.store", c.Store).Ability("settings:write").
		Middleware("idempotent")
	routing.Get(r, one, "widget-settings.show", c.Get).Ability("settings:read")
	routing.Patch(r, one, "widget-settings.update", c.Update).Ability("settings:write")
	routing.Delete(r, one, "widget-settings.destroy", c.Destroy).Ability("settings:write").
		Status(fhttp.StatusNoContent)
}
