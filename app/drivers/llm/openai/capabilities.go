/*
|--------------------------------------------------------------------------
| OpenAI Capabilities
|--------------------------------------------------------------------------
|
| The static table of what each model can do. Unknown models get the
| conservative row; the connection may override it.
|
*/

package openai

import (
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Capabilities answers what a model supports without a network call.
*/
func Capabilities(model string) contracts.LLMCapabilities {
	caps := contracts.LLMCapabilities{
		Tools:           true,
		Streaming:       true,
		SystemPrompt:    true,
		JSONMode:        true,
		PromptCache:     true,
		ContextTokens:   1_050_000,
		MaxOutputTokens: 128_000,
	}

	lower := strings.ToLower(model)
	if strings.HasPrefix(lower, "gpt-3") || strings.HasPrefix(lower, "gpt-4-turbo") {
		caps.ContextTokens = 272_000
		caps.PromptCache = false
	}

	return caps
}
