/*
|--------------------------------------------------------------------------
| Workflow Simulation Requests
|--------------------------------------------------------------------------
|
| A workflow's saved simulations: reading, saving, changing and
| deleting them.
|
*/

package requests

/*
WorkflowSimulationRequest names one saved simulation of a workflow.
*/
type WorkflowSimulationRequest struct {
	Workflow   string `path:"workflow"`
	Simulation string `path:"simulation"`
}

/*
StoreWorkflowSimulationRequest is the input of POST
/workflows/{workflow}/simulations.
*/
type StoreWorkflowSimulationRequest struct {
	Workflow string `path:"workflow"`
	Body     TestCaseBody
}

/*
UpdateWorkflowSimulationRequest is the input of PATCH
/workflows/{workflow}/simulations/{simulation}.
*/
type UpdateWorkflowSimulationRequest struct {
	Workflow   string `path:"workflow"`
	Simulation string `path:"simulation"`
	Body       TestCaseBody
}
