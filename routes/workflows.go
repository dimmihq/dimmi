/*
|--------------------------------------------------------------------------
| Workflow Routes
|--------------------------------------------------------------------------
|
| The API of a workspace's workflows. Every member may read them, to
| understand why a conversation went where it went; changing them takes
| the workflows:write ability, and the actions check the member may
| manage workflows.
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
workflowRoutes declares the workflow API.
*/
func workflowRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Workflows == nil {
		return
	}

	wf := &v1.WorkflowController{Workflows: a.Actions.Workflows, IDs: a.IDs}
	one := "/workflows/{workflow}"

	routing.Get(workspace, "/workflows", "workflows.index", wf.Index).Ability("workflows:read")
	routing.Post(workspace, "/workflows", "workflows.store", wf.Store).
		Ability("workflows:write").Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/workflows", "workflows.update-many", wf.Reorder).Ability("workflows:write")
	routing.Get(workspace, one, "workflows.show", wf.Show).Ability("workflows:read")
	routing.Patch(workspace, one, "workflows.update", wf.Update).Ability("workflows:write")
	routing.Delete(workspace, one, "workflows.destroy", wf.Destroy).
		Ability("workflows:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, one+"/unarchive", "workflows.unarchive.store", wf.Unarchive).
		Ability("workflows:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/publications", "workflows.publications.store", wf.Publish).
		Ability("workflows:write").Status(fhttp.StatusOK)
	routing.Get(workspace, one+"/versions", "workflows.versions.index", wf.Versions).Ability("workflows:read")
}
