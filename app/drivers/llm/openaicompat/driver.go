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
	"net/http"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
Config carries the connection's quirks: which max-tokens field the
server honors, whether tools work, and whether stream usage arrives.
*/
type Config struct {
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
func New(baseURL, apiKey string, headers map[string]string, httpClient *http.Client, cfg Config) *Driver {
	return &Driver{
		inner: openai.New(openai.Options{BaseURL: baseURL, APIKey: apiKey, Headers: headers, HTTP: httpClient}),
		cfg:   cfg,
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
Probe sends the smallest request to a model.
*/
func (d *Driver) Probe(ctx context.Context, model string) (*contracts.ProbeResult, error) {
	return d.inner.Probe(ctx, model)
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
