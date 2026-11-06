/*
|--------------------------------------------------------------------------
| AI Actions
|--------------------------------------------------------------------------
|
| The agent's business actions: its settings, its provider
| connections and the ChatGPT sign-in. The pipeline and the run and
| spend readers join here as they land.
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

	ListProviderKeys   *ai.ListProviderKeys
	CreateProviderKey  *ai.CreateProviderKey
	UpdateProviderKey  *ai.UpdateProviderKey
	DeleteProviderKey  *ai.DeleteProviderKey
	TestProviderKey    *ai.TestProviderKey

	StartChatGPTConnect    *ai.StartChatGPTConnect
	CompleteChatGPTConnect *ai.CompleteChatGPTConnect
}
