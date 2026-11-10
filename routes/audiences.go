/*
|--------------------------------------------------------------------------
| Audience Routes
|--------------------------------------------------------------------------
|
| The API of saved audiences, previews and tests. Reading, previewing and
| testing take the audiences:read ability; changing takes
| audiences:write. Tagging customers in bulk, from a list or by freezing
| an audience, takes contacts:write. The actions check the member's permissions, and
| testing a person also needs leave to read contacts.
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
audienceRoutes declares the audiences API.
*/
func audienceRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Audiences == nil {
		return
	}

	ac := &v1.AudienceController{Audiences: a.Actions.Audiences, IDs: a.IDs}
	pc := &v1.AudiencePreviewController{Audiences: a.Actions.Audiences, IDs: a.IDs}
	one := "/audiences/{audience}"

	routing.Get(workspace, "/audiences", "audiences.index", ac.Index).Ability("audiences:read").
		Lists(v1.PageListing()).Query(numberedPagesQuery()...)
	routing.Post(workspace, "/audiences", "audiences.store", ac.Store).
		Ability("audiences:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "audiences.show", ac.Show).Ability("audiences:read")
	routing.Patch(workspace, one, "audiences.update", ac.Update).Ability("audiences:write")
	routing.Delete(workspace, one, "audiences.destroy", ac.Destroy).
		Ability("audiences:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, one+"/previews", "audiences.previews.store", pc.PreviewSaved).
		Ability("audiences:read").Status(fhttp.StatusOK)
	routing.Post(workspace, "/audience-previews", "audience-previews.store", pc.Preview).
		Ability("audiences:read").Status(fhttp.StatusOK)
	routing.Post(workspace, "/audience-evaluations", "audience-evaluations.store", pc.Evaluate).
		Ability("audiences:read").Status(fhttp.StatusOK)
	routing.Get(workspace, "/audience-fields", "audience-fields.index", pc.Fields).Ability("audiences:read")

	lists := &v1.ContactListController{Lists: a.Actions.ContactLists, Audiences: a.Actions.Audiences, IDs: a.IDs}
	routing.Post(workspace, one+"/freezes", "audiences.freezes.store", lists.Freeze).
		Ability("contacts:write").Status(fhttp.StatusOK)
	routing.Post(workspace, "/contact-lists", "contact-lists.store", lists.Import).
		Ability("contacts:write").Status(fhttp.StatusOK)
}
