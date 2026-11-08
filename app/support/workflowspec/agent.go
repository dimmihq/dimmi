/*
|--------------------------------------------------------------------------
| Agent Handling
|--------------------------------------------------------------------------
|
| While a run waits on the step that lets dimmi handle a conversation,
| its stored wait carries what the agent must honor on every turn: the
| step's extra guidance, its reply limit and whether answers ask the
| customer to confirm. The agent reads it from the run itself, so the
| step's settings are never copied anywhere that could drift.
|
*/

package workflowspec

import "encoding/json"

/*
WaitAgent is the wait kind of a run whose agent step is active.
*/
const WaitAgent = "dimmi"

/*
AgentHandling is what an active agent step tells the agent: extra
instructions, the most replies it may send (zero for the agent's own
limit) and whether answers ask the customer to confirm them.
*/
type AgentHandling struct {
	Instructions      string `json:"instructions,omitempty"`
	MaxReplies        int    `json:"max_replies,omitempty"`
	ConfirmResolution bool   `json:"confirm_resolution"`
}

/*
AgentHandlingOf reads the agent step's settings from a run's stored
wait, reporting whether the run waits on an agent step at all.
*/
func AgentHandlingOf(wait json.RawMessage) (AgentHandling, bool) {
	var w struct {
		Kind  string         `json:"kind"`
		Agent *AgentHandling `json:"dimmi"`
	}

	if json.Unmarshal(wait, &w) != nil || w.Kind != WaitAgent || w.Agent == nil {
		return AgentHandling{}, false
	}

	return *w.Agent, true
}
