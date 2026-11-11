/*
|--------------------------------------------------------------------------
| Knowledge Source Routes
|--------------------------------------------------------------------------
|
| The API of knowledge sources. Reading takes settings:read; adding,
| changing and removing sources, and syncing them, take settings:write,
| and the actions check the finer knowledge permissions behind it.
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
knowledgeSourceRoutes declares the knowledge sources API.
*/
func knowledgeSourceRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.KnowledgeSources == nil {
		return
	}

	c := &v1.KnowledgeSourceController{Engine: a.Actions.KnowledgeSources, IDs: a.IDs}
	one := "/knowledge/sources/{source}"

	routing.Get(workspace, "/knowledge/source-types", "knowledge.source-types.index", c.Types).Ability("settings:read")
	routing.Get(workspace, "/knowledge/sources", "knowledge.sources.index", c.Index).Ability("settings:read")
	routing.Get(workspace, "/knowledge/usage", "knowledge.usage.show", c.Usage).Ability("settings:read")
	routing.Post(workspace, "/knowledge/sources", "knowledge.sources.store", c.Store).
		Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Post(workspace, "/knowledge/sources/previews", "knowledge.sources.previews.store", c.Preview).
		Ability("settings:write").Status(fhttp.StatusOK)
	routing.Post(workspace, "/knowledge/sources/trees", "knowledge.sources.trees.store", c.Tree).
		Ability("settings:write").Status(fhttp.StatusOK)
	routing.Post(workspace, "/knowledge/sources/options", "knowledge.sources.options.store", c.Options).
		Ability("settings:write").Status(fhttp.StatusOK)
	routing.Post(workspace, "/knowledge/uploads", "knowledge.uploads.store", c.Upload).
		Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "knowledge.sources.show", c.Show).Ability("settings:read")
	routing.Get(workspace, one+"/tree", "knowledge.sources.tree.show", c.SourceTree).Ability("settings:read")
	routing.Patch(workspace, one, "knowledge.sources.update", c.Update).Ability("settings:write")
	routing.Delete(workspace, one, "knowledge.sources.destroy", c.Destroy).
		Ability("settings:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, one+"/imports", "knowledge.sources.imports.store", c.Convert).
		Ability("settings:write").Status(fhttp.StatusAccepted)

	knowledgeSyncRoutes(workspace, c, one)
}

/*
knowledgeSyncRoutes declares a source's syncs and documents.
*/
func knowledgeSyncRoutes(workspace *routing.Router, c *v1.KnowledgeSourceController, one string) {
	doc := one + "/documents/{document}"

	routing.Get(workspace, one+"/syncs", "knowledge.sources.syncs.index", c.Runs).Ability("settings:read")
	routing.Post(workspace, one+"/syncs", "knowledge.sources.syncs.store", c.Sync).
		Ability("settings:write").Status(fhttp.StatusAccepted)
	routing.Delete(workspace, one+"/syncs/{run}", "knowledge.sources.syncs.destroy", c.Cancel).
		Ability("settings:write").Status(fhttp.StatusOK)
	routing.Get(workspace, one+"/documents", "knowledge.sources.documents.index", c.Documents).
		Ability("settings:read")
	routing.Post(workspace, doc+"/syncs", "knowledge.sources.documents.syncs.store", c.ResyncDocument).
		Ability("settings:write").Status(fhttp.StatusAccepted)
	routing.Post(workspace, doc+"/exclusion", "knowledge.sources.documents.exclusion.store", c.ExcludeDocument).
		Ability("settings:write").Status(fhttp.StatusNoContent)
	routing.Delete(workspace, doc+"/exclusion", "knowledge.sources.documents.exclusion.destroy", c.IncludeDocument).
		Ability("settings:write").Status(fhttp.StatusOK)
}
