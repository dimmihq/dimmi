/*
|--------------------------------------------------------------------------
| AI Agent Simulation Run Requests
|--------------------------------------------------------------------------
|
| Running an agent's simulations against its draft or its live version,
| and saving a real conversation as a simulation.
|
*/

package requests

/*
AIRunAgentSimulationsBody names the simulations to run, every one when
none is named, the copy they run against (draft by default, or live),
and the stream their progress goes out on.
*/
type AIRunAgentSimulationsBody struct {
	SimulationIDs []string `json:"simulation_ids,omitempty" maxItems:"20"`
	Version       string   `json:"version,omitempty"        enum:"draft,live,"`
	StreamID      string   `json:"stream_id,omitempty"      maxLength:"64"`
}

/*
AIRunAgentSimulationsRequest is the input of POST
/ai/agents/{agent}/simulations/run.
*/
type AIRunAgentSimulationsRequest struct {
	Agent string `path:"agent"`
	Body  *AIRunAgentSimulationsBody
}

/*
AIImportAgentSimulationBody names the conversation to save as a
simulation.
*/
type AIImportAgentSimulationBody struct {
	ConversationID string `json:"conversation_id" validate:"required,max=64"`
}

/*
AIImportAgentSimulationRequest is the input of POST
/ai/agents/{agent}/simulations/from-conversation.
*/
type AIImportAgentSimulationRequest struct {
	Agent string `path:"agent"`
	Body  AIImportAgentSimulationBody
}
