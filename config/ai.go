/*
|--------------------------------------------------------------------------
| AI Settings
|--------------------------------------------------------------------------
|
| Everything the agent needs that is not a workspace's own choice:
| the kill switch, the built-in provider's upstream credentials, the
| subscription sign-in endpoints, budgets, debounce, retention, the
| self-host bootstrap keys, the built-in embedder, retrieval timeouts
| and the directory of seed articles the agent answers from until the
| help center's index exists.
|
*/

package config

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
	Decisions AIDecisionsConfig `koanf:"decisions"`
	Knowledge AIKnowledgeConfig `koanf:"knowledge"`

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

	KnowledgeSeedPath string `koanf:"knowledge_seed_path"`
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

	Embeddings AIManagedEmbeddings `koanf:"embeddings"`
}

/*
AIManagedUpstream is one upstream the built-in provider routes to.
*/
type AIManagedUpstream struct {
	APIKey  string `koanf:"api_key"`
	BaseURL string `koanf:"base_url"`
}

/*
AIChatGPTConfig configures the subscription sign-in source. It signs
in with the public client the ChatGPT coding tools share, so a hosted
web app cannot use its own redirect: the member either enters a
device code on OpenAI's page or pastes the address the sign-in lands
on. Issuer hosts the device-code endpoints; RedirectURI is the
loopback address registered for that client; APIBaseURL is the
subscription's model endpoint; Originator names this app to it.
Enabled stays off until the operator turns the source on.
*/
type AIChatGPTConfig struct {
	Enabled      bool   `default:"false"                                  koanf:"enabled"`
	ClientID     string `default:"app_EMoamEEZ73f0CkXaXp7hrann"           koanf:"client_id"`
	Issuer       string `default:"https://auth.openai.com"                koanf:"issuer"`
	AuthorizeURL string `default:"https://auth.openai.com/oauth/authorize" koanf:"authorize_url"`
	TokenURL     string `default:"https://auth.openai.com/oauth/token"     koanf:"token_url"`
	RedirectURI  string `default:"http://localhost:1455/auth/callback"     koanf:"redirect_uri"`
	APIBaseURL   string `default:"https://chatgpt.com/backend-api/codex"   koanf:"api_base_url"`
	Originator   string `default:"dimmi"                                  koanf:"originator"`
}
