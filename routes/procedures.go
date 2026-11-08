/*
|--------------------------------------------------------------------------
| Procedure Routes
|--------------------------------------------------------------------------
|
| The API of the procedures dimmi follows. Reading them takes the ai:read
| ability; changing and publishing take ai:write, and the actions check
| the member may manage the agent.
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
procedureRoutes declares the procedures API.
*/
func procedureRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Procedures == nil {
		return
	}

	pc := &v1.ProcedureController{Procedures: a.Actions.Procedures, IDs: a.IDs}
	one := "/procedures/{procedure}"

	routing.Get(workspace, "/procedures", "procedures.index", pc.Index).Ability("ai:read")
	routing.Post(workspace, "/procedures", "procedures.store", pc.Store).
		Ability("ai:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "procedures.show", pc.Show).Ability("ai:read")
	routing.Patch(workspace, one, "procedures.update", pc.Update).Ability("ai:write")
	routing.Delete(workspace, one, "procedures.destroy", pc.Destroy).
		Ability("ai:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, one+"/publications", "procedures.publications.store", pc.Publish).
		Ability("ai:write").Status(fhttp.StatusOK)
	routing.Delete(workspace, one+"/publications", "procedures.publications.destroy", pc.Unpublish).
		Ability("ai:write").Status(fhttp.StatusOK)
}
