/*
|--------------------------------------------------------------------------
| Anthropic Capabilities
|--------------------------------------------------------------------------
|
| The static table of what each model can do.
|
*/

package anthropic

import (
	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Capabilities answers what a model supports without a network call.
*/
func Capabilities(model string) contracts.LLMCapabilities {
	return contracts.LLMCapabilities{
		Tools:           true,
		Streaming:       true,
		SystemPrompt:    true,
		JSONMode:        false,
		PromptCache:     true,
		ContextTokens:   200_000,
		MaxOutputTokens: 64_000,
	}
}
