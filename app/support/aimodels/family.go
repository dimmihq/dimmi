/*
|--------------------------------------------------------------------------
| Model Families
|--------------------------------------------------------------------------
|
| A model's family decides the prompt set it reads, while the
| provider says whose credentials the call spends. The managed
| provider fronts several upstreams, so its family comes from the
| catalog entry the model belongs to.
|
*/

package aimodels

import (
	"strings"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
FamilyOf resolves a model's family for a provider's model id. The
catalog knows the managed list; other providers fall back to their
name's prefix.
*/
func FamilyOf(provider enums.AIProvider, model string) enums.AIModelFamily {
	if provider == enums.AIProviderDimmi {
		if e := Find(Managed(), model); e != nil {
			return e.Family
		}
		return familyOfManaged(model)
	}

	switch provider {
	case enums.AIProviderAnthropic:
		return enums.AIModelFamilyAnthropic
	case enums.AIProviderOpenAI, enums.AIProviderChatGPT:
		return enums.AIModelFamilyOpenAI
	case enums.AIProviderDimmi:
		return familyOfManaged(model)
	case enums.AIProviderEngy, enums.AIProviderOpenAICompatible:
		return familyOfManaged(model)
	}
	return enums.AIModelFamilyOpenWeight
}

/*
Find returns the catalog entry for a model id, or nil.
*/
func Find(models []Entry, id string) *Entry {
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	return nil
}

/*
familyOfManaged guesses a family from the id's prefix for models the
catalog does not list.
*/
func familyOfManaged(model string) enums.AIModelFamily {
	if strings.HasPrefix(model, "claude") {
		return enums.AIModelFamilyAnthropic
	}
	if strings.HasPrefix(model, "gpt") || strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") {
		return enums.AIModelFamilyOpenAI
	}
	return enums.AIModelFamilyOpenWeight
}
