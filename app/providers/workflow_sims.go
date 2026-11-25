/*
|--------------------------------------------------------------------------
| Workflow Simulation Wiring
|--------------------------------------------------------------------------
|
| Workflow simulations walk the engine's dry runs, play the customer and
| judge the checks with the procedures' simulator, and answer dimmi's
| turns inside agent steps on the agent simulator.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/actions/workflowsims"
)

/*
bootWorkflowSims builds the workflow simulations once the engine and
the simulator are there.
*/
func bootWorkflowSims(a *app.App, simulator procedures.Simulator) {
	if a.Actions.WorkflowEngine == nil {
		return
	}
	a.Actions.WorkflowSims = &workflowsims.Simulations{
		DB: a.DB, Clock: a.Clock, Engine: a.Actions.WorkflowEngine, Simulator: simulator,
		Procedures: a.Actions.Procedures,
	}
}
