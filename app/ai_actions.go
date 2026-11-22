/*
|--------------------------------------------------------------------------
| AI Actions
|--------------------------------------------------------------------------
|
| The agents' business actions: the agents and the workspace's shared
| AI settings, the provider connections, the ChatGPT sign-in, the run
| and usage readers, and the workspace actions an agent may call with
| their connections, calls and the visitor's confirmations.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/actions/agentsims"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
)

/*
AIActions groups the agent's actions.
*/
type AIActions struct {
	Agents      *ai.Agents
	Simulations *agentsims.Simulations

	ListProviderKeys  *ai.ListProviderKeys
	CreateProviderKey *ai.CreateProviderKey
	UpdateProviderKey *ai.UpdateProviderKey
	DeleteProviderKey *ai.DeleteProviderKey
	TestProviderKey   *ai.TestProviderKey

	StartChatGPTConnect    *ai.StartChatGPTConnect
	CompleteChatGPTConnect *ai.CompleteChatGPTConnect

	ListRuns  *ai.ListConversationRuns
	ShowRun   *ai.ShowRun
	ShowUsage *ai.ShowUsage

	ActionKit         *actionkit.Kit
	ActionConnections *agentactions.Connections
	ActionDefinitions *agentactions.Definitions
	ActionCalls       *agentactions.Calls
	VisitorCalls      *widget.VisitorActionCalls
}
