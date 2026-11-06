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
	return []Entry{
		{ID: "claude-sonnet-5-5", DisplayName: "Claude Sonnet 5.5", Family: enums.AIModelFamilyAnthropic, ContextTokens: 200_000, Suggested: []enums.AIModelRole{enums.AIModelRoleAnswer}},
		{ID: "claude-haiku-5-5", DisplayName: "Claude Haiku 5.5", Family: enums.AIModelFamilyAnthropic, ContextTokens: 200_000, Suggested: []enums.AIModelRole{enums.AIModelRoleClassifier}},
		{ID: "gpt-5.2", DisplayName: "GPT-5.2", Family: enums.AIModelFamilyOpenAI, ContextTokens: 400_000, Suggested: []enums.AIModelRole{enums.AIModelRoleAnswer}},
		{ID: "gpt-5-mini", DisplayName: "GPT-5 mini", Family: enums.AIModelFamilyOpenAI, ContextTokens: 400_000, Suggested: []enums.AIModelRole{enums.AIModelRoleClassifier}},
		{ID: "glm-5.2", DisplayName: "GLM 5.2", Family: enums.AIModelFamilyOpenWeight, ContextTokens: 200_000, Suggested: []enums.AIModelRole{enums.AIModelRoleAnswer}},
		{ID: "kimi-k3", DisplayName: "Kimi K3", Family: enums.AIModelFamilyOpenWeight, ContextTokens: 1_000_000, Suggested: []enums.AIModelRole{enums.AIModelRoleAnswer}},
	}
}

/*
ChatGPT lists the models a subscription connection typically offers;
the live list still comes from the account where possible.
*/
func ChatGPT() []Entry {
	return []Entry{
		{ID: "gpt-5.2", DisplayName: "GPT-5.2", Family: enums.AIModelFamilyOpenAI, ContextTokens: 400_000, Suggested: []enums.AIModelRole{enums.AIModelRoleAnswer}},
		{ID: "gpt-5-mini", DisplayName: "GPT-5 mini", Family: enums.AIModelFamilyOpenAI, ContextTokens: 400_000, Suggested: []enums.AIModelRole{enums.AIModelRoleClassifier}},
	}
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
