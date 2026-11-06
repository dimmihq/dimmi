/*
|--------------------------------------------------------------------------
| Model Catalog
|--------------------------------------------------------------------------
|
| The models the picker offers per provider: id, name, family,
| context, and which roles each suits. Managed models live in one
| curated list because the built-in provider fronts several
| upstreams at once.
|
*/

package aimodels

import (
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
Entry is one selectable model.
*/
type Entry struct {
	ID            string
	DisplayName   string
	Family        enums.AIModelFamily
	ContextTokens int
	Suggested     []enums.AIModelRole
}

/*
Managed lists the curated models the built-in provider offers,
spanning every family so a workspace needs no key at all.
*/
func Managed() []Entry {
	return append(append(Anthropic(), OpenAI()...), Engy()...)
}

/*
answerRole is the one suggestion every launch model carries.
*/
func answerRole() []enums.AIModelRole {
	return []enums.AIModelRole{enums.AIModelRoleAnswer}
}

/*
Anthropic lists the Claude models.
*/
func Anthropic() []Entry {
	return []Entry{
		{
			ID: "claude-sonnet-5-5", DisplayName: "Claude Sonnet 5.5",
			Family: enums.AIModelFamilyAnthropic, ContextTokens: 1_000_000, Suggested: answerRole(),
		},
		{
			ID: "claude-opus-5-5", DisplayName: "Claude Opus 5.5",
			Family: enums.AIModelFamilyAnthropic, ContextTokens: 1_000_000, Suggested: answerRole(),
		},
		{
			ID: "claude-haiku-5-5", DisplayName: "Claude Haiku 5.5",
			Family: enums.AIModelFamilyAnthropic, ContextTokens: 1_000_000, Suggested: answerRole(),
		},
	}
}

/*
OpenAI lists the GPT models.
*/
func OpenAI() []Entry {
	return []Entry{
		{
			ID: "gpt-6.1-sol", DisplayName: "GPT-6.1 Sol",
			Family: enums.AIModelFamilyOpenAI, ContextTokens: 1_050_000, Suggested: answerRole(),
		},
		{
			ID: "gpt-6.1-terra", DisplayName: "GPT-6.1 Terra",
			Family: enums.AIModelFamilyOpenAI, ContextTokens: 1_050_000, Suggested: answerRole(),
		},
		{
			ID: "gpt-6-luna", DisplayName: "GPT-6 Luna",
			Family: enums.AIModelFamilyOpenAI, ContextTokens: 1_050_000, Suggested: answerRole(),
		},
	}
}

/*
Engy lists the open-weight models.
*/
func Engy() []Entry {
	return []Entry{
		{
			ID: "glm-5.3-flash", DisplayName: "GLM 5.3 Flash",
			Family: enums.AIModelFamilyOpenWeight, ContextTokens: 327_680, Suggested: answerRole(),
		},
	}
}

/*
ChatGPT lists the models a subscription connection typically offers;
the live list still comes from the account where possible.
*/
func ChatGPT() []Entry {
	return OpenAI()
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
