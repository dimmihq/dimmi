/*
|--------------------------------------------------------------------------
| AI Agent Simulation Requests
|--------------------------------------------------------------------------
|
| Trying an agent as a customer would: one preview turn, and the saved
| simulations it must keep getting right.
|
*/

package requests

/*
AIPreviewAgentBody is one preview turn: which copy answers (draft by
default, or live), the conversation so far or a real one to replay,
the customer's message, the persona, the mocks, and the state and
waiting writes the last turn handed back.
*/
type AIPreviewAgentBody struct {
	Version              string                     `json:"version,omitempty"                enum:"draft,live,"`
	History              []SimMessageInput          `json:"history,omitempty"                maxItems:"40"`
	Message              string                     `json:"message,omitempty"                maxLength:"4000"`
	Persona              *SimPersonaInput           `json:"persona,omitempty"`
	ActionMocks          map[string]ActionMockInput `json:"action_mocks,omitempty"`
	ActiveState          *SimStateInput             `json:"active_state,omitempty"`
	Confirmed            []string                   `json:"confirmed_action_ids,omitempty"   maxItems:"10"`
	SourceConversationID string                     `json:"source_conversation_id,omitempty"`
	StreamID             string                     `json:"stream_id,omitempty"              maxLength:"64"`
}

/*
AIPreviewAgentRequest is the input of POST /ai/agents/{agent}/preview.
*/
type AIPreviewAgentRequest struct {
	Agent string `path:"agent"`
	Body  AIPreviewAgentBody
}

/*
AIAgentSimulationRequest names one saved simulation of an agent.
*/
type AIAgentSimulationRequest struct {
	Agent      string `path:"agent"`
	Simulation string `path:"simulation"`
}

/*
AIStoreAgentSimulationRequest is the input of POST
/ai/agents/{agent}/simulations.
*/
type AIStoreAgentSimulationRequest struct {
	Agent string `path:"agent"`
	Body  TestCaseBody
}

/*
AIUpdateAgentSimulationRequest is the input of PATCH
/ai/agents/{agent}/simulations/{simulation}.
*/
type AIUpdateAgentSimulationRequest struct {
	Agent      string `path:"agent"`
	Simulation string `path:"simulation"`
	Body       TestCaseBody
}
