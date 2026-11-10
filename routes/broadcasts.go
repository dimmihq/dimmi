/*
|--------------------------------------------------------------------------
| Broadcast Routes
|--------------------------------------------------------------------------
|
| The API of broadcasts and their topics. Reading takes broadcasts:read,
| writing drafts and topics broadcasts:write, and anything that reaches
| customers broadcasts:write with the send permission the action checks.
| The preference page and the links inside broadcast emails need no
| sign-in: each carries a signed token naming its own recipient.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
)

/*
broadcastRoutes declares the broadcasts API.
*/
func broadcastRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Broadcasts == nil {
		return
	}

	bc := &v1.BroadcastController{Broadcasts: a.Actions.Broadcasts, IDs: a.IDs}
	tc := &v1.BroadcastTopicController{Broadcasts: a.Actions.Broadcasts, IDs: a.IDs}
	one := "/broadcasts/{broadcast}"

	routing.Get(workspace, "/broadcasts", "broadcasts.index", bc.Index).Ability("broadcasts:read")
	routing.Post(workspace, "/broadcasts", "broadcasts.store", bc.Store).
		Ability("broadcasts:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "broadcasts.show", bc.Show).Ability("broadcasts:read")
	routing.Patch(workspace, one, "broadcasts.update", bc.Update).Ability("broadcasts:write")
	routing.Delete(workspace, one, "broadcasts.destroy", bc.Destroy).
		Ability("broadcasts:write").Status(fhttp.StatusNoContent)
	routing.Get(workspace, one+"/report", "broadcasts.report.show", bc.Report).Ability("broadcasts:read")
	routing.Get(workspace, one+"/recipients", "broadcasts.recipients.index", bc.Recipients).
		Ability("broadcasts:read")
	routing.Get(workspace, one+"/estimate", "broadcasts.estimate.show", bc.Estimate).Ability("broadcasts:read")

	broadcastActionRoutes(workspace, bc, one)

	routing.Post(workspace, one+"/tests", "broadcasts.tests.store", bc.Test).
		Ability("broadcasts:write").Status(fhttp.StatusNoContent)

	routing.Get(workspace, "/broadcast-topics", "broadcast-topics.index", tc.Index).Ability("broadcasts:read")
	routing.Post(workspace, "/broadcast-topics", "broadcast-topics.store", tc.Store).
		Ability("broadcasts:write").Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/broadcast-topics/{topic}", "broadcast-topics.update", tc.Update).
		Ability("broadcasts:write")
}

/*
broadcastActionRoutes declares what can be done to one broadcast.
*/
func broadcastActionRoutes(workspace *routing.Router, bc *v1.BroadcastController, one string) {
	routing.Post(workspace, one+"/send", "broadcasts.send", bc.Send).
		Ability("broadcasts:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/pause", "broadcasts.pause", bc.Pause).
		Ability("broadcasts:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/resume", "broadcasts.resume", bc.Resume).
		Ability("broadcasts:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/stop", "broadcasts.stop", bc.Stop).
		Ability("broadcasts:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/cancel", "broadcasts.cancel", bc.Cancel).
		Ability("broadcasts:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/duplicate", "broadcasts.duplicate", bc.Duplicate).
		Ability("broadcasts:write").Status(fhttp.StatusCreated)
}

/*
broadcastPreferenceRoutes declares the preference page's API, reached
without signing in.
*/
func broadcastPreferenceRoutes(guest *routing.Router, a *app.App) {
	if a.Actions.Broadcasts == nil {
		return
	}

	pc := &v1.BroadcastPreferencesController{Broadcasts: a.Actions.Broadcasts, IDs: a.IDs}

	routing.Get(guest, "/broadcast-preferences/{token}", "broadcast-preferences.show", pc.Show)
	routing.Put(guest, "/broadcast-preferences/{token}", "broadcast-preferences.update", pc.Update)
}

/*
broadcastLinks declares the links inside broadcast emails.
*/
func broadcastLinks(a *app.App) {
	if a.Actions.Broadcasts == nil {
		return
	}

	links := web.BroadcastLinkController{Broadcasts: a.Actions.Broadcasts, AppURL: a.Application.Config.App.URL}

	a.Router.Handle(fhttp.MethodGet, "/b/o/{token}", "broadcasts.links.open", fhttp.HandlerFunc(links.Open))
	a.Router.Handle(fhttp.MethodGet, "/b/c/{token}", "broadcasts.links.click", fhttp.HandlerFunc(links.Click))
	a.Router.Handle(fhttp.MethodGet, "/b/u/{token}", "broadcasts.links.preferences",
		fhttp.HandlerFunc(links.Preferences))
	a.Router.Handle(fhttp.MethodPost, "/b/u/{token}/one-click", "broadcasts.links.one-click",
		fhttp.HandlerFunc(links.OneClick))
}
