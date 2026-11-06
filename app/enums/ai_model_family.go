/*
|--------------------------------------------------------------------------
| AI Model Family
|--------------------------------------------------------------------------
|
| Models that think alike share a family, and each family gets its own
| tuned prompt files: one for Anthropic's models, one for OpenAI's,
| one for the open-weight models. Which family a model belongs to is
| decided by a static table, never by asking the provider.
|
*/

package enums

/*
AIModelFamily names a group of models sharing one prompt tuning.
*/
type AIModelFamily string

/*
The model families.
*/
const (
	AIModelFamilyAnthropic  AIModelFamily = "anthropic"
	AIModelFamilyOpenAI     AIModelFamily = "openai"
	AIModelFamilyOpenWeight AIModelFamily = "openweight"
)

/*
AIModelFamilies returns every family.
*/
func AIModelFamilies() []AIModelFamily {
	return []AIModelFamily{AIModelFamilyAnthropic, AIModelFamilyOpenAI, AIModelFamilyOpenWeight}
}
