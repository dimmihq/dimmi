/*
|--------------------------------------------------------------------------
| OpenAI-Compatible Driver
|--------------------------------------------------------------------------
|
| Any endpoint that speaks the OpenAI wire — Ollama, LM Studio, vLLM,
| or a hosted gateway — answers through this driver. It shares the
| OpenAI mapping and only differs in what the connection supplies:
| a base URL, an optional key, and the quirks the key's config notes.
|
*/

package openaicompat

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
Config carries the connection's quirks: which max-tokens field the
server honors, whether tools work, and whether stream usage arrives.
*/
type Config struct {
	Clock               clock.Clock
	MaxTokensField      string
	SupportsTools       bool
	SupportsStreamUsage bool
	JSONMode            bool
	ContextTokens       int
}

/*
Driver is a contracts.LLM over an OpenAI-compatible endpoint.
*/
type Driver struct {
	inner *openai.Driver
	cfg   Config
}

/*
New builds a compatible driver for the endpoint.
*/
/*
Options configures the compatible endpoint.
*/

/*
Options configures the compatible endpoint.
*/
type Options struct {
	BaseURL, APIKey string
	Headers         map[string]string
	HTTP            *httpclient.StdClient
	Config          Config
}

/*
New builds the compatible driver.
*/
func New(o Options) *Driver {
	return &Driver{
		inner: openai.New(openai.Options{
			BaseURL: o.BaseURL, APIKey: o.APIKey, Headers: o.Headers, HTTP: o.HTTP, Clock: o.Config.Clock,
		}),
		cfg: o.Config,
	}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderOpenAICompatible }

/*
Chat runs one non-streaming completion.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	return d.inner.Chat(ctx, req)
}

/*
Stream runs one streaming completion.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	return d.inner.Stream(ctx, req)
}

/*
ListModels returns the endpoint's models.
*/
func (d *Driver) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	return d.inner.ListModels(ctx)
}

/*
Capabilities answers from the connection's config.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	caps := d.inner.Capabilities(model)
	caps.Tools = d.cfg.SupportsTools
	if d.cfg.ContextTokens > 0 {
		caps.ContextTokens = d.cfg.ContextTokens
	}
	return caps
}
