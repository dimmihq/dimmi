/*
|--------------------------------------------------------------------------
| AI Provider
|--------------------------------------------------------------------------
|
| A provider names which driver a connection's credentials speak to:
| the built-in Dimmi provider, a model company, a subscription
| sign-in, or any OpenAI-compatible endpoint. The set grows one
| driver at a time, so the database only checks a value's shape.
|
*/

package enums

/*
AIProvider names the driver behind a provider connection.
*/
type AIProvider string

/*
The providers a connection can point at today.
*/
const (
	AIProviderDimmi            AIProvider = "dimmi"
	AIProviderAnthropic        AIProvider = "anthropic"
	AIProviderOpenAI           AIProvider = "openai"
	AIProviderChatGPT          AIProvider = "chatgpt"
	AIProviderEngy             AIProvider = "engy"
	AIProviderOpenAICompatible AIProvider = "openai_compatible"
)

/*
AIProviders returns every provider, in display order.
*/
func AIProviders() []AIProvider {
	return []AIProvider{
		AIProviderDimmi, AIProviderAnthropic, AIProviderOpenAI, AIProviderChatGPT, AIProviderEngy,
		AIProviderOpenAICompatible,
	}
}
