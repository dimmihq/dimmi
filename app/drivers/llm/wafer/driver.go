/*
|--------------------------------------------------------------------------
| Wafer Driver
|--------------------------------------------------------------------------
|
| Wafer hosts the open-weight models behind an OpenAI-compatible API.
| dimmi names a model the same way whichever host serves it, so the
| driver turns dimmi's names into Wafer's own, such as glm-5.3-flash
| into GLM-5.3-Flash, and passes any other name through untouched.
| Wafer caches repeated prompt prefixes and prices cached input lower,
| so the driver says the prompt is cached.
|
*/

package wafer

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
	"github.com/dimmihq/dimmi/app/drivers/llm/openaicompat"
)

/*
BaseURL is Wafer's model API endpoint.
*/
const BaseURL = "https://pass.wafer.ai/v1"

/*
Options configures one Wafer connection; an empty BaseURL means
Wafer's own endpoint.
*/
type Options struct {
	BaseURL string
	APIKey  string
	HTTP    *httpclient.StdClient
	Clock   clock.Clock
}

/*
Driver is a contracts.LLM over Wafer.
*/
type Driver struct {
	*openaicompat.Driver
}

/*
New builds a Wafer driver for a key.
*/
func New(o Options) *Driver {
	if o.BaseURL == "" {
		o.BaseURL = BaseURL
	}

	return &Driver{Driver: openaicompat.New(openaicompat.Options{
		Provider: contracts.LLMProviderWafer, BaseURL: o.BaseURL, APIKey: o.APIKey,
		HTTP: o.HTTP, Clock: o.Clock,
		Config: openaicompat.Config{
			MaxTokensField: openai.MaxTokensLegacy, SupportsTools: true, SupportsStreamUsage: true,
			PromptCache: true,
		},
	})}
}

/*
Chat runs one completion with the model under Wafer's name.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	req.Model = ModelID(req.Model)

	return d.Driver.Chat(ctx, req)
}

/*
Stream streams one completion with the model under Wafer's name.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	req.Model = ModelID(req.Model)

	return d.Driver.Stream(ctx, req)
}

/*
Capabilities answers for the model under Wafer's name.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	caps := d.Driver.Capabilities(ModelID(model))
	caps.ContextTokens, caps.MaxOutputTokens = window(model)

	return caps
}

/*
ModelID turns dimmi's name for an open-weight model into Wafer's; any
other name stays as it is.
*/
func ModelID(model string) string {
	if id, ok := models()[model]; ok {
		return id
	}

	return model
}

/*
models maps dimmi's model names to the ones Wafer serves them under.
*/
func models() map[string]string {
	return map[string]string{
		"glm-5.3-flash":     "GLM-5.3-Flash",
		"glm-5.3":           "GLM-5.3",
		"glm-5.2":           "GLM-5.2",
		"kimi-k3":           "Kimi-K3",
		"deepseek-v4-flash": "DeepSeek-V4.1-Flash",
	}
}

/*
window is a hosted model's context window and output bound.
*/
func window(model string) (contextTokens, maxOutput int) {
	switch {
	case strings.Contains(strings.ToLower(model), "glm-5.3"):
		return 327_680, 32_768
	case strings.Contains(strings.ToLower(model), "glm-5.2"):
		return 262_144, 32_768
	case strings.Contains(strings.ToLower(model), "deepseek-v4"):
		return 1_048_576, 32_768
	}

	return 128_000, 8_192
}
