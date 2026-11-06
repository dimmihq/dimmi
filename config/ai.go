/*
|--------------------------------------------------------------------------
| AI Settings
|--------------------------------------------------------------------------
|
| Everything the agent needs that is not a workspace's own choice:
| the kill switch, the built-in provider's upstream credentials, the
| subscription sign-in endpoints, budgets, debounce, retention and
| the self-host bootstrap keys.
|
*/

package config

import "time"

/*
AIConfig configures the AI agent.
*/
type AIConfig struct {
	Enabled bool `default:"true" koanf:"enabled"`

	Managed AIManagedConfig `koanf:"managed"`
	ChatGPT AIChatGPTConfig `koanf:"chatgpt"`

	Prices    AIPricesConfig    `koanf:"prices"`
	Prompts   AIPromptsConfig   `koanf:"prompts"`
	Timeouts  AITimeoutsConfig  `koanf:"timeouts"`
	Retry     AIRetryConfig     `koanf:"retry"`
	Breaker   AIBreakerConfig   `koanf:"breaker"`
	Bootstrap AIBootstrapConfig `koanf:"bootstrap"`

	TracesRetentionDays     int  `default:"30"  koanf:"traces_retention_days"`
	RunsRetentionDays       int  `default:"400" koanf:"runs_retention_days"`
	PlaygroundRetentionDays int  `default:"30" koanf:"playground_retention_days"`
	TracesStorePayloads     bool `default:"true" koanf:"traces_store_payloads"`

	DebounceWidgetMS int `default:"1500" koanf:"debounce_widget_ms"`
	DebounceChatMS   int `default:"2500" koanf:"debounce_chat_ms"`
	DebounceEmailMS  int `default:"0"    koanf:"debounce_email_ms"`

	WorkspaceConcurrency   int     `default:"3"   koanf:"workspace_concurrency"`
	ContactDailyRuns       int     `default:"100" koanf:"contact_daily_runs"`
	ConversationHourlyRuns int     `default:"30"  koanf:"conversation_hourly_runs"`
	DefaultMonthlyCapUSD   float64 `default:"25" koanf:"default_monthly_cap_usd"`

	KnowledgeStub bool `default:"false" koanf:"knowledge_stub"`
}

/*
AIManagedConfig holds the built-in provider's upstream credentials:
one key per model family, all optional so an install can run any
subset. On Dimmi Cloud these are Dimmi's own keys; on self-host the
operator's, and absent means the built-in provider is hidden.
*/
type AIManagedConfig struct {
	Anthropic  AIManagedUpstream `koanf:"anthropic"`
	OpenAI     AIManagedUpstream `koanf:"openai"`
	OpenWeight AIManagedUpstream `koanf:"openweight"`
}

/*
AIManagedUpstream is one upstream the built-in provider routes to.
*/
type AIManagedUpstream struct {
	APIKey  string `koanf:"api_key"`
	BaseURL string `koanf:"base_url"`
}

/*
AIChatGPTConfig configures the subscription sign-in source. Enabled
stays off until the install has an OAuth client id; the hosted cloud
needs the registered client while self-host registers itself.
*/
type AIChatGPTConfig struct {
	Enabled      bool   `default:"false"                               koanf:"enabled"`
	ClientID     string `koanf:"client_id"`
	AuthorizeURL string `default:"https://auth.openai.com/oauth/authorize" koanf:"authorize_url"`
	TokenURL     string `default:"https://auth.openai.com/oauth/token"     koanf:"token_url"`
	APIBaseURL   string `default:"https://api.openai.com/v1"             koanf:"api_base_url"`
}

/*
AIPricesConfig names where the model price table lives.
*/
type AIPricesConfig struct {
	Path     string `koanf:"path"`
	SyncURL  string `koanf:"sync_url"`
	AutoSync bool   `default:"false" koanf:"auto_sync"`
}

/*
AIPromptsConfig selects the prompt version and, on self-host, an
override directory.
*/
type AIPromptsConfig struct {
	Version string `koanf:"version"`
	Path    string `koanf:"path"`
}

/*
AITimeoutsConfig bounds each pipeline step's provider call.
*/
type AITimeoutsConfig struct {
	ClassifyTotal   time.Duration `default:"8s"  koanf:"classify_total"`
	GenerateFirst   time.Duration `default:"12s" koanf:"generate_first_token"`
	GenerateIdle    time.Duration `default:"15s" koanf:"generate_idle"`
	GenerateTotal   time.Duration `default:"60s" koanf:"generate_total"`
	VerifyTotal     time.Duration `default:"10s" koanf:"verify_total"`
	SummarizeTotal  time.Duration `default:"10s" koanf:"summarize_total"`
	ValidationTotal time.Duration `default:"10s" koanf:"validation_total"`
	ProbeTotal      time.Duration `default:"20s" koanf:"probe_total"`
}

/*
AIRetryConfig bounds the retry wrapper.
*/
type AIRetryConfig struct {
	MaxAttempts int `default:"3" koanf:"max_attempts"`
}

/*
AIBreakerConfig sets the per-connection circuit breaker.
*/
type AIBreakerConfig struct {
	Failures int           `default:"5"    koanf:"failures"`
	Window   time.Duration `default:"60s" koanf:"window"`
	Open     time.Duration `default:"60s" koanf:"open"`
}

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
