/*
|--------------------------------------------------------------------------
| Procedure Simulate Requests
|--------------------------------------------------------------------------
|
| One simulated turn: the conversation so far or a real one to replay,
| the customer's message, and the state and waiting writes the last turn
| handed back.
|
*/

package requests

/*
SimPendingInput is a write the last simulated turn left waiting.
*/
type SimPendingInput struct {
	ActionID string         `json:"action_id"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args,omitempty"`
	State    string         `json:"state,omitempty"`
	Summary  string         `json:"summary,omitempty"`
}

/*
SimStateInput is the procedure state the last simulated turn handed
back.
*/
type SimStateInput struct {
	ProcedureID     string            `json:"procedure_id,omitempty"`
	Status          string            `json:"status,omitempty"`
	CalledActionIDs []string          `json:"called_action_ids,omitempty"`
	CompletedSteps  []string          `json:"completed_step_ids,omitempty"`
	Branches        map[string]string `json:"branches,omitempty"`
	StepOutputs     map[string]any    `json:"step_outputs,omitempty"`
	Pending         []SimPendingInput `json:"pending,omitempty"`
}

/*
SimulateProcedureRequest is the input of POST
/procedures/{procedure}/simulate: one turn against the working copy, or
the live one, with the conversation so far or a real one to replay.
*/
type SimulateProcedureRequest struct {
	Procedure string `path:"procedure"`
	Body      struct {
		Target               string                     `json:"target,omitempty"                 enum:"draft,live,"`
		History              []SimMessageInput          `json:"history,omitempty"                maxItems:"40"`
		Message              string                     `json:"message,omitempty"                maxLength:"4000"`
		Persona              *SimPersonaInput           `json:"persona,omitempty"`
		ActionMocks          map[string]ActionMockInput `json:"action_mocks,omitempty"`
		ActiveState          *SimStateInput             `json:"active_state,omitempty"`
		Confirmed            []string                   `json:"confirmed_action_ids,omitempty"   maxItems:"10"`
		SourceConversationID string                     `json:"source_conversation_id,omitempty"`
		StreamID             string                     `json:"stream_id,omitempty"              maxLength:"64"`
	}
}
