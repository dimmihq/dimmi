/*
|--------------------------------------------------------------------------
| AI Actions
|--------------------------------------------------------------------------
|
| The agent's business actions: its settings, its provider
| connections, the ChatGPT sign-in, the run and usage readers, the
| playground, and the workspace actions it may call with their
| connections, calls and the visitor's confirmations.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
)

/*
AIActions groups the agent's actions.
*/
type AIActions struct {
	GetAgent    *ai.GetAiAgent
	UpdateAgent *ai.UpdateAiAgent

	ListProviderKeys  *ai.ListProviderKeys
	CreateProviderKey *ai.CreateProviderKey
	UpdateProviderKey *ai.UpdateProviderKey
	DeleteProviderKey *ai.DeleteProviderKey
	TestProviderKey   *ai.TestProviderKey

	StartChatGPTConnect    *ai.StartChatGPTConnect
	CompleteChatGPTConnect *ai.CompleteChatGPTConnect

	ListRuns      *ai.ListConversationRuns
	ShowRun       *ai.ShowRun
	ShowUsage     *ai.ShowUsage
	AskPlayground *ai.AskPlayground

	ActionKit         *actionkit.Kit
	ActionConnections *agentactions.Connections
	ActionDefinitions *agentactions.Definitions
	ActionCalls       *agentactions.Calls
	VisitorCalls      *widget.VisitorActionCalls
}
