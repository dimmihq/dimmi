/*
|--------------------------------------------------------------------------
| Anthropic Capabilities
|--------------------------------------------------------------------------
|
| The static table of what each Claude model can do: its context
| window, its output bound, and whether it still takes sampling
| settings. The current models refuse a temperature outright, so the
| driver leaves it out for them rather than fail the call.
|
*/

package anthropic

import (
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
model is one row of the table, matched by id prefix.
*/
type model struct {
	prefix    string
	context   int
	maxOutput int
	sampling  bool
}

/*
models lists the rows, longest prefixes first so a family's newer
members win over its older ones.
*/
func models() []model {
	return []model{
		{prefix: "claude-fable-5", context: 1_000_000, maxOutput: 128_000},
		{prefix: "claude-mythos-5", context: 1_000_000, maxOutput: 128_000},
		{prefix: "claude-opus-5", context: 1_000_000, maxOutput: 128_000},
		{prefix: "claude-sonnet-5", context: 1_000_000, maxOutput: 128_000},
		{prefix: "claude-haiku-5", context: 1_000_000, maxOutput: 128_000},
		{prefix: "claude-opus-4-8", context: 1_000_000, maxOutput: 128_000},
		{prefix: "claude-opus-4-7", context: 1_000_000, maxOutput: 128_000},
		{prefix: "claude-opus-4-6", context: 1_000_000, maxOutput: 128_000, sampling: true},
		{prefix: "claude-sonnet-4-6", context: 1_000_000, maxOutput: 128_000, sampling: true},
		{prefix: "claude-haiku-4-5", context: 200_000, maxOutput: 64_000, sampling: true},
	}
}

/*
fallbackModel is assumed for a model the table does not know: the
smaller window, and no sampling settings, which every model accepts.
*/
func fallbackModel() model {
	return model{context: 200_000, maxOutput: 64_000}
}

/*
lookup finds a model's row.
*/
func lookup(id string) model {
	for _, m := range models() {
		if strings.HasPrefix(id, m.prefix) {
			return m
		}
	}
	return fallbackModel()
}

/*
Capabilities answers what a model supports without a network call.
*/
func Capabilities(id string) contracts.LLMCapabilities {
	m := lookup(id)
	return contracts.LLMCapabilities{
		Tools:           true,
		Streaming:       true,
		SystemPrompt:    true,
		JSONSchema:      true,
		PromptCache:     true,
		ContextTokens:   m.context,
		MaxOutputTokens: m.maxOutput,
	}
}

/*
acceptsSampling reports whether a model takes a temperature.
*/
func acceptsSampling(id string) bool {
	return lookup(id).sampling
}
