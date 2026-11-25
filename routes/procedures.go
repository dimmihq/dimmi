/*
|--------------------------------------------------------------------------
| Procedure Routes
|--------------------------------------------------------------------------
|
| The API of the procedures dimmi follows. Reading them, their saved
| tests, versions and numbers takes the ai:read ability; changing,
| publishing, simulating and running tests take ai:write, and the
| actions check the member may manage the agent.
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

	routing.Post(workspace, one+"/simulate", "procedures.simulate.store", pc.Simulate).
		Ability("ai:write").Status(fhttp.StatusOK)

	tests := one + "/test-cases"
	routing.Get(workspace, tests, "procedures.test-cases.index", pc.TestCases).Ability("ai:read")
	routing.Post(workspace, tests, "procedures.test-cases.store", pc.StoreTestCase).
		Ability("ai:write").Status(fhttp.StatusCreated)
	routing.Post(workspace, tests+"/run", "procedures.test-cases.run.store", pc.RunTestCases).
		Ability("ai:write").Status(fhttp.StatusOK)
	routing.Get(workspace, tests+"/{test_case}", "procedures.test-cases.show", pc.ShowTestCase).Ability("ai:read")
	routing.Patch(workspace, tests+"/{test_case}", "procedures.test-cases.update", pc.UpdateTestCase).
		Ability("ai:write")
	routing.Delete(workspace, tests+"/{test_case}", "procedures.test-cases.destroy", pc.DestroyTestCase).
		Ability("ai:write").Status(fhttp.StatusNoContent)

	versions := one + "/versions"
	routing.Get(workspace, versions, "procedures.versions.index", pc.Versions).Ability("ai:read")
	routing.Get(workspace, versions+"/{version}", "procedures.versions.show", pc.ShowVersion).Ability("ai:read")
	routing.Post(workspace, versions+"/{version}/restore", "procedures.versions.restore.store", pc.RestoreVersion).
		Ability("ai:write").Status(fhttp.StatusOK)

	routing.Get(workspace, one+"/metrics", "procedures.metrics.show", pc.Metrics).Ability("ai:read")
	routing.Get(workspace, "/reports/procedures", "reports.procedures.index", pc.Report).Ability("ai:read")
}
