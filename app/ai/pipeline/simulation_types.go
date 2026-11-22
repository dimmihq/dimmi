/*
|--------------------------------------------------------------------------
| Simulated Conversations
|--------------------------------------------------------------------------
|
| What a simulated conversation carries from turn to turn: its earlier
| messages, the made-up customer, and the procedure's state, with the
| writes that wait on the customer or a teammate.
|
*/

package pipeline

/*
SimMessage is one earlier message of a simulated conversation: who
it is from, customer or agent, and what it said.
*/
type SimMessage struct {
	From string `json:"role"`
	Text string `json:"text"`
}

/*
SimPersona is the made-up customer: who they are, whether they are
signed in, where they write from, and their trusted attributes and
company's.
*/
type SimPersona struct {
	Name       string         `json:"name,omitempty"`
	Email      string         `json:"email,omitempty"`
	ExternalID string         `json:"external_id,omitempty"`
	Verified   bool           `json:"verified,omitempty"`
	Channel    string         `json:"channel,omitempty"`
	InboxID    string         `json:"inbox_id,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Company    map[string]any `json:"company,omitempty"`
}

/*
SimPending is a write waiting on the customer or a teammate.
*/
type SimPending struct {
	ActionID string         `json:"action_id"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args"`
	State    string         `json:"state"`
	Summary  string         `json:"summary"`
}

/*
SimState is the procedure state a simulated conversation carries from
turn to turn.
*/
type SimState struct {
	ProcedureID string            `json:"procedure_id,omitempty"`
	Status      string            `json:"status,omitempty"`
	Called      []string          `json:"called_action_ids"`
	Completed   []string          `json:"completed_step_ids"`
	Branches    map[string]string `json:"branches"`
	Outputs     map[string]any    `json:"step_outputs"`
	Pending     []SimPending      `json:"pending"`
}
