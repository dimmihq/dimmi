/*
|--------------------------------------------------------------------------
| LLM Driver Registry
|--------------------------------------------------------------------------
|
| The launch set of providers, each one factory turning a key row and
| its credentials into a driver: the key-driven providers read the
| row's endpoint and config, the subscription source refreshes its own
| bearer, and the built-in provider fronts whichever upstreams the
| install configured, the open-weight models on two hosts in turn.
|
*/

package managers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/anthropic"
	"github.com/dimmihq/dimmi/app/drivers/llm/baseten"
	"github.com/dimmihq/dimmi/app/drivers/llm/chatgpt"
	"github.com/dimmihq/dimmi/app/drivers/llm/engy"
	"github.com/dimmihq/dimmi/app/drivers/llm/failover"
	"github.com/dimmihq/dimmi/app/drivers/llm/managed"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
	"github.com/dimmihq/dimmi/app/drivers/llm/openaicompat"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/llmretry"
)

/*
registerBuiltins registers the drivers of the launch set.
*/
func (m *LLMManager) registerBuiltins() {
	m.Extend(enums.AIProviderAnthropic, func(
		_ context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.LLM, error) {
		return anthropic.New(anthropic.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey, Clock: m.clock,
			Version: keyConfig(key).AnthropicVersion, HTTP: hc.StdClient(),
		}), nil
	})

	m.Extend(enums.AIProviderOpenAI, func(
		_ context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.LLM, error) {
		return openai.New(openai.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey, Headers: creds.ExtraHeaders,
			HTTP: hc.StdClient(), Clock: m.clock,
		}), nil
	})

	m.registerOpenWeight()

	m.Extend(enums.AIProviderOpenAICompatible, func(
		_ context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.LLM, error) {
		return openaicompat.New(openaicompat.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey, Headers: creds.ExtraHeaders,
			HTTP: hc.StdClient(), Clock: m.clock, Config: keyConfig(key).compat(),
		}), nil
	})

	m.Extend(enums.AIProviderChatGPT, func(
		_ context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.LLM, error) {
		return chatgpt.New(chatgpt.Options{
			BaseURL: m.config.ChatGPT.APIBaseURL, Originator: m.config.ChatGPT.Originator,
			HTTP: hc.StdClient(), Source: &oauthSource{m: m, key: key, creds: creds}, Clock: m.clock,
		}), nil
	})

	m.Extend(enums.AIProviderDimmi, func(
		_ context.Context, m *LLMManager, _ models.AiProviderKey, _ Credentials, hc *httpclient.Client,
	) (contracts.LLM, error) {
		return m.managedDriver(hc)
	})
}

/*
registerOpenWeight registers the hosts of the open-weight models that
take a workspace's own key.
*/
func (m *LLMManager) registerOpenWeight() {
	m.Extend(enums.AIProviderEngy, func(
		_ context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.LLM, error) {
		return engy.New(engy.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey, HTTP: hc.StdClient(), Clock: m.clock,
		}), nil
	})

	m.Extend(enums.AIProviderBaseten, func(
		_ context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.LLM, error) {
		return baseten.New(baseten.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey, HTTP: hc.StdClient(), Clock: m.clock,
		}), nil
	})
}

/*
managedDriver builds the multiplexer from the instance's managed
credentials, offering only the models of the configured families.
*/
func (m *LLMManager) managedDriver(hc *httpclient.Client) (contracts.LLM, error) {
	cfg := m.config.Managed
	families := make(map[enums.AIModelFamily]contracts.LLM)

	if cfg.Anthropic.APIKey != "" {
		families[enums.AIModelFamilyAnthropic] = anthropic.New(anthropic.Options{
			BaseURL: cfg.Anthropic.BaseURL, APIKey: cfg.Anthropic.APIKey, HTTP: hc.StdClient(), Clock: m.clock,
		})
	}
	if cfg.OpenAI.APIKey != "" {
		families[enums.AIModelFamilyOpenAI] = openai.New(openai.Options{
			BaseURL: cfg.OpenAI.BaseURL, APIKey: cfg.OpenAI.APIKey, HTTP: hc.StdClient(), Clock: m.clock,
		})
	}
	if cfg.OpenWeight.Configured() {
		families[enums.AIModelFamilyOpenWeight] = m.openWeightHosts(hc)
	}

	if len(families) == 0 {
		return nil, fmt.Errorf("managed provider not configured")
	}

	var offered []contracts.LLMModel
	for _, e := range m.ManagedModels() {
		offered = append(offered, contracts.LLMModel{
			ID: e.ID, DisplayName: e.DisplayName, ContextTokens: e.ContextTokens,
		})
	}

	return managed.New(families, offered), nil
}

/*
openWeightHosts fronts the hosts the built-in provider sends the
open-weight models to: Baseten first and Engy when Baseten cannot
serve, or whichever of the two the install has a key for. Each host
has its own breaker per model, so a host that keeps failing is
skipped until it recovers.
*/
func (m *LLMManager) openWeightHosts(hc *httpclient.Client) contracts.LLM {
	cfg := m.config.Managed.OpenWeight
	var hosts []contracts.LLM

	if cfg.Baseten.APIKey != "" {
		hosts = append(hosts, baseten.New(baseten.Options{
			BaseURL: cfg.Baseten.BaseURL, APIKey: cfg.Baseten.APIKey, HTTP: hc.StdClient(), Clock: m.clock,
		}))
	}
	if cfg.Engy.APIKey != "" {
		hosts = append(hosts, engy.New(engy.Options{
			BaseURL: cfg.Engy.BaseURL, APIKey: cfg.Engy.APIKey, HTTP: hc.StdClient(), Clock: m.clock,
		}))
	}

	return failover.New(failover.Options{
		Hosts: hosts,
		BreakerFor: func(host contracts.LLMProvider, model string) *llmretry.Breaker {
			return m.breakers.For("managed:" + string(host) + ":" + model)
		},
	})
}

/*
connectionConfig is the shape of a key's config blob: the Anthropic
version header and an OpenAI-compatible server's quirks, each
optional.
*/
type connectionConfig struct {
	AnthropicVersion    string `json:"anthropic_version"`
	MaxTokensField      string `json:"max_tokens_field"`
	SupportsTools       *bool  `json:"supports_tools"`
	SupportsStreamUsage *bool  `json:"supports_stream_usage"`
	JSONSchema          *bool  `json:"json_schema"`
	ContextTokens       int    `json:"context_tokens"`
	MaxOutputTokens     int    `json:"max_output_tokens"`
}

/*
keyConfig reads a key's config blob; an unreadable blob reads as
empty.
*/
func keyConfig(key models.AiProviderKey) connectionConfig {
	var cfg connectionConfig
	_ = json.Unmarshal(key.Config, &cfg)
	return cfg
}

/*
compat maps the blob onto the compatible driver's defaults.
*/
func (c connectionConfig) compat() openaicompat.Config {
	out := openaicompat.DefaultConfig()
	if c.MaxTokensField != "" {
		out.MaxTokensField = c.MaxTokensField
	}
	if c.SupportsTools != nil {
		out.SupportsTools = *c.SupportsTools
	}
	if c.SupportsStreamUsage != nil {
		out.SupportsStreamUsage = *c.SupportsStreamUsage
	}
	if c.JSONSchema != nil {
		out.JSONSchema = *c.JSONSchema
	}
	out.ContextTokens = c.ContextTokens
	out.MaxOutputTokens = c.MaxOutputTokens
	return out
}

/*
str dereferences a nullable string.
*/
func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
