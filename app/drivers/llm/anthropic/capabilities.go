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
func Capabilities(_ string) contracts.LLMCapabilities {
	return contracts.LLMCapabilities{
		Tools:           true,
		Streaming:       true,
		SystemPrompt:    true,
		JSONMode:        false,
		PromptCache:     true,
		ContextTokens:   1_000_000,
		MaxOutputTokens: 128_000,
	}
}
