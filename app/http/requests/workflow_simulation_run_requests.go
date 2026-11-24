/*
|--------------------------------------------------------------------------
| Workflow Simulation Run Requests
|--------------------------------------------------------------------------
|
| Running a workflow's simulations against its draft or its live
| version, reading a simulation's past runs, and saving a real
| conversation as a simulation.
|
*/

package requests

/*
RunWorkflowSimulationsBody names the simulations to run, every one when
none is named, the copy they run against (draft by default, or live),
and the stream their progress goes out on.
*/
type RunWorkflowSimulationsBody struct {
	SimulationIDs []string `json:"simulation_ids,omitempty" maxItems:"20"`
	Version       string   `json:"version,omitempty"        enum:"draft,live,"`
	StreamID      string   `json:"stream_id,omitempty"      maxLength:"64"`
}

/*
RunWorkflowSimulationsRequest is the input of POST
/workflows/{workflow}/simulations/run.
*/
type RunWorkflowSimulationsRequest struct {
	Workflow string `path:"workflow"`
	Body     *RunWorkflowSimulationsBody
}

/*
ImportWorkflowSimulationBody names the conversation to save as a
simulation.
*/
type ImportWorkflowSimulationBody struct {
	ConversationID string `json:"conversation_id" validate:"required,max=64"`
}

/*
ImportWorkflowSimulationRequest is the input of POST
/workflows/{workflow}/simulations/from-conversation.
*/
type ImportWorkflowSimulationRequest struct {
	Workflow string `path:"workflow"`
	Body     ImportWorkflowSimulationBody
}

/*
WorkflowSimulationRunsRequest is the input of GET
/workflows/{workflow}/simulations/{simulation}/runs. Version is the copy
whose runs to list, the draft when left out.
*/
type WorkflowSimulationRunsRequest struct {
	Workflow   string `path:"workflow"`
	Simulation string `path:"simulation"`
	Version    string `query:"version" enum:"draft,live" required:"false"`
}
