/*
|--------------------------------------------------------------------------
| AI Limits
|--------------------------------------------------------------------------
|
| How the agent is priced, prompted and kept within bounds: the price
| table's source, the prompt set, each step's provider timeouts, the
| retry budget and the circuit breaker that stops a failing connection
| from costing real calls.
|
*/

package config

import "time"

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
AITimeoutsConfig bounds each pipeline step's provider call. The retry
wrapper enforces the bounds a request carries, so a step passes its
row through llmretry.StepTimeouts into ChatRequest.Timeout.
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
AIRetryConfig bounds the retry wrapper: how many attempts one call
gets and the longest provider retry hint it honors before giving up.
*/
type AIRetryConfig struct {
	MaxAttempts   int           `default:"3"   koanf:"max_attempts"`
	MaxRetryAfter time.Duration `default:"20s" koanf:"max_retry_after"`
}

/*
AIBreakerConfig sets the per-connection circuit breaker.
*/
type AIBreakerConfig struct {
	Failures int           `default:"5"    koanf:"failures"`
	Window   time.Duration `default:"60s" koanf:"window"`
	Open     time.Duration `default:"60s" koanf:"open"`
}
