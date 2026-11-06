/*
|--------------------------------------------------------------------------
| AI Actions
|--------------------------------------------------------------------------
|
| The agent's business actions: its settings, its provider
| connections, the ChatGPT sign-in, the run and usage readers and the
| playground.
|
*/

package app

import "github.com/dimmihq/dimmi/app/actions/ai"

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
}
