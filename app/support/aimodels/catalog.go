/*
|--------------------------------------------------------------------------
| Model Catalog
|--------------------------------------------------------------------------
|
| The models the picker offers per provider: id, name, family, context
| window, output bound, and which roles each suits. Managed models live
| in one curated list because the built-in provider fronts several
| upstreams at once. A ChatGPT plan reaches the same GPT models with a
| smaller window than the API, so it has its own rows.
|
*/

package aimodels

import (
	"slices"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
Entry is one selectable model.
*/
type Entry struct {
	ID              string
	DisplayName     string
	Family          enums.AIModelFamily
	ContextTokens   int
	MaxOutputTokens int
	Suggested       []enums.AIModelRole
}

/*
Managed lists the curated models the built-in provider offers,
spanning every family so a workspace needs no key at all.
*/
func Managed() []Entry {
	return append(append(Anthropic(), OpenAI()...), Engy()...)
}

/*
ManagedFor lists the managed models whose family the install has an
upstream for, so the picker never offers a model nothing can serve.
*/
func ManagedFor(configured func(enums.AIModelFamily) bool) []Entry {
	var out []Entry
	for _, e := range Managed() {
		if configured(e.Family) {
			out = append(out, e)
		}
	}
	return out
}

/*
answerRole is the one suggestion every launch model carries.
*/
func answerRole() []enums.AIModelRole {
	return []enums.AIModelRole{enums.AIModelRoleAnswer}
}

/*
claude builds a Claude row; every current Claude model has a 1M-token
window and a 128K output bound.
*/
func claude(id, name string) Entry {
	return Entry{
		ID: id, DisplayName: name, Family: enums.AIModelFamilyAnthropic,
		ContextTokens: 1_000_000, MaxOutputTokens: 128_000, Suggested: answerRole(),
	}
}

/*
Anthropic lists the Claude models dimmi supports. Every provider's list
is deliberately short: only models that were tested with the agent are
offered, cheapest first, so the first one is a sensible default.
*/
func Anthropic() []Entry {
	return []Entry{
		claude("claude-haiku-5-5", "Claude Haiku 5.5"),
		claude("claude-sonnet-5-5", "Claude Sonnet 5.5"),
	}
}

/*
gpt builds a GPT row with the given window.
*/
func gpt(id, name string, contextTokens int) Entry {
	return Entry{
		ID: id, DisplayName: name, Family: enums.AIModelFamilyOpenAI,
		ContextTokens: contextTokens, MaxOutputTokens: 128_000, Suggested: answerRole(),
	}
}

/*
OpenAI lists the GPT models of the API dimmi supports.
*/
func OpenAI() []Entry {
	return []Entry{
		gpt("gpt-6-luna", "GPT-6 Luna", 1_050_000),
		gpt("gpt-6.1-terra", "GPT-6.1 Terra", 1_050_000),
	}
}

/*
Engy lists the open-weight models.
*/
func Engy() []Entry {
	return []Entry{
		{
			ID: "glm-5.3-flash", DisplayName: "GLM 5.3 Flash", Family: enums.AIModelFamilyOpenWeight,
			ContextTokens: 327_680, MaxOutputTokens: 32_768, Suggested: answerRole(),
		},
	}
}

/*
ChatGPT lists the supported models a ChatGPT plan offers, with the
window a plan allows.
*/
func ChatGPT() []Entry {
	return []Entry{
		gpt("gpt-6-luna", "GPT-6 Luna", 272_000),
		gpt("gpt-6.1-terra", "GPT-6.1 Terra", 500_000),
	}
}

/*
Supported lists the models dimmi offers on a provider. An
OpenAI-compatible server runs whatever its operator installed, so it
has no list and every model it reports is kept.
*/
func Supported(provider enums.AIProvider) ([]Entry, bool) {
	switch provider {
	case enums.AIProviderAnthropic:
		return Anthropic(), true
	case enums.AIProviderOpenAI:
		return OpenAI(), true
	case enums.AIProviderEngy, enums.AIProviderBaseten:
		return Engy(), true
	case enums.AIProviderChatGPT:
		return ChatGPT(), true
	case enums.AIProviderDimmi:
		return Managed(), true
	case enums.AIProviderOpenAICompatible:
		return nil, false
	}
	return nil, false
}

/*
KeepSupported narrows the models a provider reported to the ones dimmi
supports, in the catalog's order.
*/
func KeepSupported(provider enums.AIProvider, reported []string) []string {
	catalog, limited := Supported(provider)
	if !limited {
		return reported
	}

	kept := []string{}
	for _, e := range catalog {
		if slices.Contains(reported, e.ID) {
			kept = append(kept, e.ID)
		}
	}
	return kept
}

/*
Suggested picks the default answer and classifier models from a
model list, preferring entries marked for those roles.
*/
func Suggested(models []contracts.LLMModel) (answer, classifier string) {
	for _, m := range models {
		if answer == "" {
			answer = m.ID
		}
	}
	return answer, answer
}
