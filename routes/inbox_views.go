/*
|--------------------------------------------------------------------------
| Inbox View Routes
|--------------------------------------------------------------------------
|
| How each teammate arranges their inbox: their own preferences for the
| sidebar and every view, and the saved views the team shares or each
| member keeps. A preference is the caller's own, never data others
| read, so reading and saving it takes conversations:read; saving views
| takes conversations:write.
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
inboxViewRoutes declares the routes of inbox preferences.
*/
func inboxViewRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.Inbox
	prefs := &v1.InboxPreferenceController{Show: acts.ShowPreferences, Update: acts.UpdatePreferences}

	routing.Get(r, "/inbox-preferences", "inbox-preferences.show", prefs.Get).Ability("conversations:read")
	routing.Patch(r, "/inbox-preferences", "inbox-preferences.update", prefs.Patch).Ability("conversations:read")

	views := &v1.ConversationViewController{
		List: acts.ListViews, Show: acts.ShowView, Create: acts.CreateView, Update: acts.UpdateView,
		Delete: acts.DeleteView, Reorder: acts.ReorderViews, IDs: a.IDs,
	}
	view := "/conversation-views/{view}"

	routing.Get(r, "/conversation-views", "conversation-views.index", views.Index).Ability("conversations:read")
	routing.Post(r, "/conversation-views", "conversation-views.store", views.Store).Ability("conversations:write")
	routing.Patch(r, "/conversation-views", "conversation-views.reorder", views.ReorderShared).
		Ability("conversations:write")
	routing.Get(r, view, "conversation-views.show", views.Get).Ability("conversations:read")
	routing.Patch(r, view, "conversation-views.update", views.Patch).Ability("conversations:write")
	routing.Delete(r, view, "conversation-views.destroy", views.Destroy).Ability("conversations:write").
		Status(fhttp.StatusNoContent)
}
