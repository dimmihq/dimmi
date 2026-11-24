/*
|--------------------------------------------------------------------------
| Simulation Routes Of Workflows And Series
|--------------------------------------------------------------------------
|
| A workflow's saved simulations, mirroring an agent's: reading them
| takes workflows:read, saving and running them workflows:write, and
| the actions check the member may manage workflows. A series' dry
| simulation takes series:write: it spends on the AI paths it asks,
| though it never sends anything.
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
simulationRoutes declares the simulations of workflows and series.
*/
func simulationRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.WorkflowSims == nil {
		return
	}
	c := &v1.WorkflowSimulationController{Simulations: a.Actions.WorkflowSims, Series: a.Actions.Series, IDs: a.IDs}
	sims := "/workflows/{workflow}/simulations"

	routing.Get(workspace, sims, "workflows.simulations.index", c.Index).Ability("workflows:read")
	routing.Post(workspace, sims, "workflows.simulations.store", c.Store).
		Ability("workflows:write").Status(fhttp.StatusCreated)
	routing.Post(workspace, sims+"/run", "workflows.simulations.run.store", c.Run).
		Ability("workflows:write").Status(fhttp.StatusOK)
	routing.Post(workspace, sims+"/from-conversation", "workflows.simulations.from-conversation.store", c.Import).
		Ability("workflows:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, sims+"/{simulation}", "workflows.simulations.show", c.Show).Ability("workflows:read")
	routing.Get(workspace, sims+"/{simulation}/runs", "workflows.simulations.runs.index", c.Runs).
		Ability("workflows:read")
	routing.Patch(workspace, sims+"/{simulation}", "workflows.simulations.update", c.Update).
		Ability("workflows:write")
	routing.Delete(workspace, sims+"/{simulation}", "workflows.simulations.destroy", c.Destroy).
		Ability("workflows:write").Status(fhttp.StatusNoContent)

	if a.Actions.Series != nil {
		routing.Post(workspace, "/series/{series}/simulations/run", "series.simulations.run.store", c.SimulateSeries).
			Ability("series:write").Status(fhttp.StatusOK)
	}
}
