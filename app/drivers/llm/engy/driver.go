/*
|--------------------------------------------------------------------------
| Engy Driver
|--------------------------------------------------------------------------
|
| Engy serves open-weight models over the OpenAI wire at api.engy.ai,
| with a bearer key, a model list, usage in the final stream chunk and
| both max-tokens fields. On top of the compatible mapping it adds two
| provider facts: the context window of each hosted model, and the
| real charge it reports per request.
|
*/

package engy

import (
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openaicompat"
)

/*
BaseURL is Engy's endpoint.
*/
const BaseURL = "https://api.engy.ai/v1"

/*
Options configures one Engy connection; an empty BaseURL means Engy's
own endpoint.
*/
type Options struct {
	BaseURL string
	APIKey  string
	HTTP    *httpclient.StdClient
	Clock   clock.Clock
}

/*
Driver is a contracts.LLM over Engy.
*/
type Driver struct {
	*openaicompat.Driver
}

/*
New builds an Engy driver for a key.
*/
func New(o Options) *Driver {
	if o.BaseURL == "" {
		o.BaseURL = BaseURL
	}
	return &Driver{Driver: openaicompat.New(openaicompat.Options{
		Provider: contracts.LLMProviderEngy, BaseURL: o.BaseURL, APIKey: o.APIKey,
		HTTP: o.HTTP, Clock: o.Clock,
		Config: openaicompat.Config{
			MaxTokensField: "max_completion_tokens", SupportsTools: true,
			SupportsStreamUsage: true, JSONMode: true,
		},
	})}
}

/*
Capabilities answers from Engy's table of hosted models.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	caps := d.Driver.Capabilities(model)
	caps.ContextTokens, caps.MaxOutputTokens = window(model)
	return caps
}

/*
window is a hosted model's context window and output bound.
*/
func window(model string) (contextTokens, maxOutput int) {
	switch {
	case strings.HasPrefix(model, "glm-5.3"):
		return 327_680, 32_768
	case strings.HasPrefix(model, "glm-5.2"):
		return 262_144, 32_768
	case strings.HasPrefix(model, "kimi-k3"):
		return 1_113_088, 32_768
	case strings.HasPrefix(model, "deepseek-v4"):
		return 1_048_576, 32_768
	case strings.HasPrefix(model, "qwen3.8"):
		return 1_001_536, 32_768
	case strings.HasPrefix(model, "qwen3.6"):
		return 208_192, 32_768
	}
	return 128_000, 8_192
}

/*
ChargedMicros parses the per-request charge Engy reports, in
micro-USD, from its response header or the x_engy extension field.
*/
func ChargedMicros(headers fhttp.Header, body string) int64 {
	if v := headers.Get("X-Engy-Charged-Micro"); v != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n
		}
	}
	if _, after, ok := strings.Cut(body, `"charged_micro":`); ok {
		end := strings.IndexAny(after, ",}")
		if end > 0 {
			if n, err := strconv.ParseInt(strings.TrimSpace(after[:end]), 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}
