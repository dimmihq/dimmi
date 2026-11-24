/*
|--------------------------------------------------------------------------
| AI Agent Requests
|--------------------------------------------------------------------------
|
| The shapes of the agents API around one agent's settings: listing and
| making agents, publishing them, their versions, what they may use and
| where they answer, and the workspace's shared AI settings.
|
*/

package requests

/*
AIAgentsRequest is the input of GET /ai/agents.
*/
type AIAgentsRequest struct{}

/*
AIAgentTemplatesRequest is the input of GET /ai/agent-templates.
*/
type AIAgentTemplatesRequest struct{}

/*
AIShowAgentRequest names one agent by its public id.
*/
type AIShowAgentRequest struct {
	Agent string `path:"agent"`
}

/*
AIStoreAgentBody is the body of POST /ai/agents: the name, and the key
of the template the agent starts from; empty starts from nothing.
*/
type AIStoreAgentBody struct {
	Name     string `json:"name"               validate:"required,min=1,max=60"`
	Template string `json:"template,omitempty" validate:"omitempty,max=40"`
}

/*
AIStoreAgentRequest is the input of POST /ai/agents.
*/
type AIStoreAgentRequest struct {
	Body AIStoreAgentBody
}
