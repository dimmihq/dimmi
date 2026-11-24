/*
|--------------------------------------------------------------------------
| Dry Simulation Traces
|--------------------------------------------------------------------------
|
| What a dry simulation of a series reports: whether the person would
| enter, the steps and paths, the messages that would be sent, the
| changes that would be made, and how the journey would end.
|
*/

package series

/*
SimTrace is what a dry simulation did: whether the person would enter,
every step with the path it took, what would be sent, the changes that
would be made, whether the goal was reached, and how it ended.
*/
type SimTrace struct {
	Entered  bool         `json:"entered"`
	Reason   string       `json:"reason,omitempty"`
	Steps    []SimStep    `json:"steps"`
	Messages []SimMessage `json:"messages"`
	Actions  []SimAction  `json:"actions"`
	Goal     bool         `json:"goal_reached"`
	End      string       `json:"end"`
}

/*
SimStep is one step a dry simulation took.
*/
type SimStep struct {
	NodeID  string         `json:"node_id"`
	Kind    string         `json:"kind"`
	Path    string         `json:"path,omitempty"`
	Result  string         `json:"result"`
	Skipped bool           `json:"skipped,omitempty"`
	Detail  map[string]any `json:"detail,omitempty"`
}

/*
SimMessage is a message that would be sent: the step, its kind, who it
goes to, and its subject and body as written.
*/
type SimMessage struct {
	NodeID  string `json:"node_id"`
	Kind    string `json:"kind"`
	To      string `json:"to,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body"`
}

/*
SimAction is a change that would be made.
*/
type SimAction struct {
	NodeID string         `json:"node_id"`
	Kind   string         `json:"kind"`
	Detail map[string]any `json:"detail,omitempty"`
}
