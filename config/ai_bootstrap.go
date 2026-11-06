/*
|--------------------------------------------------------------------------
| AI Bootstrap
|--------------------------------------------------------------------------
|
| A self-hosted install can hand its first workspace provider keys and
| default models from the environment, so the agent answers from the
| first boot without a trip through the settings.
|
*/

package config

/*
AIBootstrapConfig carries self-host env keys into the first
workspace's connections at first boot.
*/
type AIBootstrapConfig struct {
	AnthropicAPIKey string `koanf:"anthropic_api_key"`
	OpenAIAPIKey    string `koanf:"openai_api_key"`
	EngyAPIKey      string `koanf:"engy_api_key"`
	EngyBaseURL     string `koanf:"engy_base_url"`
	CompatBaseURL   string `koanf:"openai_compatible_base_url"`
	CompatAPIKey    string `koanf:"openai_compatible_api_key"`
	AnswerModel     string `koanf:"answer_model"`
	ClassifierModel string `koanf:"classifier_model"`
}
