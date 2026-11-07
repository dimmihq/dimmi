/*
|--------------------------------------------------------------------------
| Help Center Routes
|--------------------------------------------------------------------------
|
| The API of a workspace's public docs. Reading articles takes
| articles:read and writing them articles:write; changing a help
| center's address and look takes settings:write, and the actions check
| the member may manage help centers.
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
helpCenterRoutes declares the help center API.
*/
func helpCenterRoutes(workspace *routing.Router, a *app.App) {
	acts := a.Actions
	if acts.HelpCenters == nil {
		return
	}
	hc := &v1.HelpCenterController{
		HelpCenters: acts.HelpCenters, Collections: acts.Collections, Addresses: acts.HelpCenterAddresses, IDs: a.IDs,
	}
	art := &v1.ArticleController{Articles: acts.Articles, IDs: a.IDs}
	one, article := "/help-centers/{help_center}", "/articles/{article}"

	routing.Get(workspace, "/help-centers", "help-centers.index", hc.Index).Ability("articles:read")
	routing.Get(workspace, one, "help-centers.show", hc.Show).Ability("articles:read")
	routing.Patch(workspace, one, "help-centers.update", hc.Update).Ability("settings:write")
	routing.Get(workspace, one+"/collections", "help-centers.collections.index", hc.IndexCollections).
		Ability("articles:read")
	routing.Post(workspace, one+"/collections", "help-centers.collections.store", hc.StoreCollection).
		Ability("articles:write").Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/collections/{collection}", "collections.update", hc.UpdateCollection).
		Ability("articles:write")
	routing.Delete(workspace, "/collections/{collection}", "collections.destroy", hc.DestroyCollection).
		Ability("articles:write").Status(fhttp.StatusNoContent)

	routing.Get(workspace, one+"/articles", "help-centers.articles.index", art.Index).Ability("articles:read")
	routing.Post(workspace, one+"/articles", "help-centers.articles.store", art.Store).
		Ability("articles:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, article, "articles.show", art.Show).Ability("articles:read")
	routing.Patch(workspace, article, "articles.update", art.Update).Ability("articles:write")
	routing.Delete(workspace, article, "articles.destroy", art.Destroy).
		Ability("articles:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, article+"/publications", "articles.publications.store", art.Publish).
		Ability("articles:write").Status(fhttp.StatusOK)
	routing.Get(workspace, article+"/versions", "articles.versions.index", art.Versions).Ability("articles:read")
}
