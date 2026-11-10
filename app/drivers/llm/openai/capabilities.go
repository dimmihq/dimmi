/*
|--------------------------------------------------------------------------
| OpenAI Capabilities
|--------------------------------------------------------------------------
|
| The static table of what each model can do. The current GPT models
| share a 1.05M-token window and a 128K output bound; older models get
| smaller rows, and a connection may still override either. Current
| GPT models enforce a strict response schema; of the older ones only
| gpt-4o and gpt-4.1 do.
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
		JSONSchema:      true,
		PromptCache:     true,
		ContextTokens:   1_050_000,
		MaxOutputTokens: 128_000,
	}

	lower := strings.ToLower(model)
	switch {
	case strings.HasPrefix(lower, "gpt-3"), strings.HasPrefix(lower, "gpt-4"):
		caps.ContextTokens = 128_000
		caps.MaxOutputTokens = 16_384
		caps.PromptCache = false
		caps.JSONSchema = strings.HasPrefix(lower, "gpt-4o") || strings.HasPrefix(lower, "gpt-4.1")
	case strings.HasPrefix(lower, "gpt-5"):
		caps.ContextTokens = 400_000
	}

	return caps
}
