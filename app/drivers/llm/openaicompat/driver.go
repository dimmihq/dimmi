/*
|--------------------------------------------------------------------------
| OpenAI-Compatible Driver
|--------------------------------------------------------------------------
|
| Any endpoint that speaks the OpenAI wire — Ollama, LM Studio, vLLM,
| or a hosted gateway — answers through this driver. It shares the
| OpenAI mapping and only differs in what the connection supplies: a
| base URL, an optional key, and the quirks the key's config notes,
| such as the max-tokens field the server reads or whether it enforces
| a response schema.
|
*/

package openaicompat

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
Config carries the connection's quirks: which max-tokens field the
server honors ("max_tokens" or "max_completion_tokens"), whether
tools work, whether usage arrives inside a stream, whether the server
enforces a strict json_schema response format, and the context window
when the connection knows it.
*/
type Config struct {
	MaxTokensField      string
	SupportsTools       bool
	SupportsStreamUsage bool
	JSONSchema          bool
	ContextTokens       int
	MaxOutputTokens     int
}

/*
DefaultConfig is what an unknown compatible server is assumed to do:
the legacy max_tokens field every server reads, tools and stream
usage, each switchable from the connection's config. Schema
enforcement is off until the connection declares it: many servers
accept json_schema and never check the answer against it.
*/
func DefaultConfig() Config {
	return Config{MaxTokensField: openai.MaxTokensLegacy, SupportsTools: true, SupportsStreamUsage: true}
}

/*
Options configures the compatible endpoint. Provider names the
connection, openai_compatible when empty.
*/
type Options struct {
	Provider contracts.LLMProvider
	BaseURL  string
	APIKey   string
	Headers  map[string]string
	HTTP     *httpclient.StdClient
	Clock    clock.Clock
	Config   Config
}

/*
Driver is a contracts.LLM over an OpenAI-compatible endpoint.
*/
type Driver struct {
	*openai.Driver
}

/*
New builds the compatible driver.
*/
func New(o Options) *Driver {
	if o.Provider == "" {
		o.Provider = contracts.LLMProviderOpenAICompatible
	}
	cfg := o.Config
	return &Driver{Driver: openai.New(openai.Options{
		Provider: o.Provider, BaseURL: o.BaseURL, APIKey: o.APIKey, Headers: o.Headers,
		HTTP: o.HTTP, Clock: o.Clock, MaxTokensField: cfg.MaxTokensField,
		NoStreamUsage: !cfg.SupportsStreamUsage,
		Capabilities:  func(model string) contracts.LLMCapabilities { return capabilities(cfg, model) },
	})}
}

/*
ListModels returns the endpoint's models; a server without the list
endpoint reports that it has none, so the caller probes instead.
*/
func (d *Driver) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	models, err := d.Driver.ListModels(ctx)
	if err == nil {
		return models, nil
	}
	if llmErr, ok := errors.AsType[*contracts.LLMError](err); ok && llmErr.Kind == contracts.LLMErrorModelNotFound {
		return nil, contracts.ErrLLMNotSupported
	}
	return nil, err
}

/*
capabilities answers from the connection's config over a conservative
base row.
*/
func capabilities(cfg Config, _ string) contracts.LLMCapabilities {
	caps := contracts.LLMCapabilities{
		Tools: cfg.SupportsTools, Streaming: true, SystemPrompt: true, JSONSchema: cfg.JSONSchema,
		ContextTokens: 128_000, MaxOutputTokens: 8_192,
	}
	if cfg.ContextTokens > 0 {
		caps.ContextTokens = cfg.ContextTokens
	}
	if cfg.MaxOutputTokens > 0 {
		caps.MaxOutputTokens = cfg.MaxOutputTokens
	}
	return caps
}
