/*
|--------------------------------------------------------------------------
| AI Agent Steps
|--------------------------------------------------------------------------
|
| Actions that decide which AI agent works a conversation, or whether
| any does. Assigning an agent pins it to the conversation, so it
| answers from the next customer message on. Turning AI off keeps every
| agent out until a teammate or a Turn on AI step lets them back in;
| turning it on lets the agent answer again after a handover. Each runs
| in the background, so a background workflow may use it.
|
*/

package workflowspec

/*
The AI agent step kinds.
*/
const (
	KindAssignAgent = "assign_ai_agent"
	KindTurnOffAI   = "turn_off_ai"
	KindTurnOnAI    = "turn_on_ai"
)

/*
agentKinds returns the AI agent step kinds.
*/
func agentKinds() []Kind {
	return []Kind{
		{Key: KindAssignAgent, Ready: true, Action: true},
		{Key: KindTurnOffAI, Ready: true, Action: true},
		{Key: KindTurnOnAI, Ready: true, Action: true},
	}
}

/*
AssignAgentSettings pins the agent with the public id AgentID to the
conversation.
*/
type AssignAgentSettings struct {
	AgentID string `json:"agent_id"`
}

/*
NoSettings is the settings of a step that takes none.
*/
type NoSettings struct{}

/*
agentChecks returns the checks of the AI agent steps.
*/
func (c *checker) agentChecks() map[string]func(Step) {
	return map[string]func(Step){
		KindAssignAgent: c.checkAssignAgent,
		KindTurnOffAI:   func(s Step) { decode[NoSettings](c, s) },
		KindTurnOnAI:    func(s Step) { decode[NoSettings](c, s) },
	}
}

/*
checkAssignAgent checks that the step names an agent; the publish
checks the agent exists.
*/
func (c *checker) checkAssignAgent(s Step) {
	a, ok := decode[AssignAgentSettings](c, s)
	if ok && a.AgentID == "" {
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: "agent_id", Message: "Pick the agent."})
	}
}
