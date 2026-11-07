/*
|--------------------------------------------------------------------------
| Agent Message Action Marks
|--------------------------------------------------------------------------
|
| A reply that proposes or reports a change carries it: the card a
| customer confirms, and the call with where it stands. Arguments, the
| decider and the result are for the team.
|
*/

package resources

import (
	"time"
)

/*
MessageAIConfirmation is the card a customer confirms or cancels: the
call, its title, the summary rendered from its arguments, its status,
when it lapses and its lines.
*/
type MessageAIConfirmation struct {
	ActionCallID string            `json:"action_call_id"`
	Title        string            `json:"title"`
	Text         string            `json:"text"`
	Status       string            `json:"status"`
	ExpiresAt    *time.Time        `json:"expires_at"`
	Details      []MessageAIDetail `json:"details"`
}

/*
MessageAIDetail is one line of a confirmation card.
*/
type MessageAIDetail struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

/*
MessageAIActionCall is a change a reply reports. Action, Arguments,
ExpiresAt, DecidedBy, DecisionReason and ResultSummary are for the team.
*/
type MessageAIActionCall struct {
	ID             string           `json:"id"`
	State          string           `json:"state"`
	Summary        string           `json:"summary"`
	Action         *MessageAIAction `json:"action,omitempty"`
	Arguments      map[string]any   `json:"arguments,omitempty"`
	ExpiresAt      *time.Time       `json:"expires_at,omitempty"`
	DecidedBy      *MessageAIPerson `json:"decided_by,omitempty"`
	DecisionReason *string          `json:"decision_reason,omitempty"`
	ResultSummary  *string          `json:"result_summary,omitempty"`
}

/*
MessageAIAction names the action a reported call ran.
*/
type MessageAIAction struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

/*
MessageAIPerson names the teammate who decided a call.
*/
type MessageAIPerson struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
