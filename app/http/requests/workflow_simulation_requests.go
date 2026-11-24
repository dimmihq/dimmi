/*
|--------------------------------------------------------------------------
| Workflow Simulation Requests
|--------------------------------------------------------------------------
|
| A workflow's saved simulations: saving, changing and deleting them,
| saving a real conversation as one, running them against the draft or
| the live version, and reading a simulation's past runs. A series
| takes a dry simulation of one person.
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

/*
SimulateSeriesPerson is the made-up person a series' dry simulation
walks: a name, an email and custom attributes.
*/
type SimulateSeriesPerson struct {
	Name       string         `json:"name,omitempty"       maxLength:"200"`
	Email      string         `json:"email,omitempty"      maxLength:"320"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

/*
RunSeriesSimulationBody is a dry simulation of a series: the copy
(draft by default, or live), the contact who travels it or a made-up
person, the events they would do, and what they would do with its
messages.
*/
type RunSeriesSimulationBody struct {
	Version   string                `json:"version,omitempty"    enum:"draft,live,"`
	ContactID string                `json:"contact_id,omitempty" maxLength:"64"`
	Person    *SimulateSeriesPerson `json:"person,omitempty"`
	Events    []string              `json:"events,omitempty"     maxItems:"20"`
	Engaged   []string              `json:"engaged,omitempty"    maxItems:"3"`
}

/*
RunSeriesSimulationRequest is the input of POST
/series/{series}/simulations/run.
*/
type RunSeriesSimulationRequest struct {
	Series string `path:"series"`
	Body   RunSeriesSimulationBody
}
