/*
|--------------------------------------------------------------------------
| Model Families
|--------------------------------------------------------------------------
|
| Which prompt tuning a model gets, what it can do, and how much it
| can read are settled here, in a static table — never by asking the
| provider. A model the table does not know falls back to the
| open-weight family and conservative capabilities.
|
*/

package aimodels

import (
	"strings"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
FamilyOf names the family a model belongs to, decided from its
provider and id. Prefixes match longest-first; an unknown model is
treated as open-weight, which asks for the simplest prompting.
*/
func FamilyOf(provider enums.AIProvider, model string) enums.AIModelFamily {
	switch provider {
	case enums.AIProviderAnthropic:
		return enums.AIModelFamilyAnthropic
	case enums.AIProviderOpenAI, enums.AIProviderChatGPT:
		return enums.AIModelFamilyOpenAI
	case enums.AIProviderEngy, enums.AIProviderOpenAICompatible:
		return enums.AIModelFamilyOpenWeight
	case enums.AIProviderDimmi:
		return familyOfManaged(model)
	}
	return familyOfManaged(model)
}

/*
familyOfManaged resolves a model on the built-in provider by its id,
since one connection fronts every family.
*/
func familyOfManaged(model string) enums.AIModelFamily {
	lower := strings.ToLower(model)

	switch {
	case strings.HasPrefix(lower, "claude"):
		return enums.AIModelFamilyAnthropic
	case strings.HasPrefix(lower, "gpt"), strings.HasPrefix(lower, "o1"), strings.HasPrefix(lower, "o3"),
		strings.HasPrefix(lower, "o4"), strings.HasPrefix(lower, "codex"):
		return enums.AIModelFamilyOpenAI
	}
	return enums.AIModelFamilyOpenWeight
}
