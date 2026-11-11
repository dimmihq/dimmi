/*
|--------------------------------------------------------------------------
| LLM Manager
|--------------------------------------------------------------------------
|
| One place builds a contracts.LLM from a provider-key row: it opens
| the sealed credentials, picks the driver for the provider, wraps it
| with retries, a circuit breaker per connection and model, and the
| step timeouts each request carries, and keeps a short-lived cache so
| a rotated key never keeps its old driver. Drivers are built per
| provider through the registry, so a new provider lands as one
| registered factory, never a manager edit.
|
*/

package managers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/llmretry"
	"github.com/dimmihq/dimmi/config"
)

/*
DriverFactory builds a driver for one key row with the decrypted
credentials and the per-key HTTP client.
*/
type DriverFactory func(
	ctx context.Context, m *LLMManager, key models.AiProviderKey,
	creds Credentials, httpClient *httpclient.Client,
) (contracts.LLM, error)

/*
LLMManager resolves and caches drivers. The cache key is the key id
with its updated_at, so a rotation or re-seal retires the old driver;
breakers outlive the cache, so a rebuilt driver inherits the state of
the one it replaced.
*/
type LLMManager struct {
	keys      *encryption.Keys
	db        *database.DB
	http      *httpclient.Client
	config    config.AIConfig
	clock     clock.Clock
	factories map[enums.AIProvider]DriverFactory
	breakers  *llmretry.Breakers
	mu        sync.Mutex
	cache     map[string]cachedDriver
	order     []string
	renewing  map[ids.UUID]*sync.Mutex
}

/*
cachedDriver is one built driver with its freshness token.
*/
type cachedDriver struct {
	llm contracts.LLM
	at  time.Time
}

const cacheLimit = 256

/*
LLMDeps are what the manager needs from the application.
*/
type LLMDeps struct {
	Keys   *encryption.Keys
	HTTP   *httpclient.Client
	Config config.AIConfig
	Clock  clock.Clock
	DB     *database.DB
}

/*
NewLLMManager builds the manager with the drivers of today
registered.
*/
func NewLLMManager(deps LLMDeps) *LLMManager {
	clk := deps.Clock
	if clk == nil {
		clk = clock.System()
	}
	m := &LLMManager{
		keys: deps.Keys, http: deps.HTTP, config: deps.Config, clock: clk, db: deps.DB,
		factories: make(map[enums.AIProvider]DriverFactory), cache: map[string]cachedDriver{},
		breakers: llmretry.NewBreakers(clk, deps.Config.Breaker), renewing: map[ids.UUID]*sync.Mutex{},
	}
	m.registerBuiltins()
	return m
}

/*
Extend registers a provider's driver factory.
*/
func (m *LLMManager) Extend(provider enums.AIProvider, factory DriverFactory) {
	m.factories[provider] = factory
}

/*
ForKey returns the driver for a provider-key row, wrapped with
retries, the connection's breakers and timeout enforcement. A caller
bounds a step by setting the request's Timeout, typically from
llmretry.StepTimeouts(config.AI.Timeouts, step).
*/
func (m *LLMManager) ForKey(ctx context.Context, key models.AiProviderKey) (contracts.LLM, error) {
	cacheKey := key.ID.String() + ":" + key.UpdatedAt.String()

	m.mu.Lock()
	if hit, ok := m.cache[cacheKey]; ok {
		m.mu.Unlock()
		return hit.llm, nil
	}
	m.mu.Unlock()

	drv, err := m.build(ctx, key)
	if err != nil {
		return nil, err
	}
	wrapped := m.wrap(key, drv)

	m.mu.Lock()
	if len(m.order) >= cacheLimit {
		delete(m.cache, m.order[0])
		m.order = m.order[1:]
	}
	m.cache[cacheKey] = cachedDriver{llm: wrapped, at: m.clock.Now()}
	m.order = append(m.order, cacheKey)
	m.mu.Unlock()

	return wrapped, nil
}

/*
build opens the key's credentials and runs its provider's factory.
*/
func (m *LLMManager) build(ctx context.Context, key models.AiProviderKey) (contracts.LLM, error) {
	factory, ok := m.factories[enums.AIProvider(key.Provider)]
	if !ok {
		return nil, fmt.Errorf("no llm driver for provider %q", key.Provider)
	}

	creds, err := OpenCredentials(ctx, m.keys, key)
	if err != nil {
		return nil, err
	}

	return factory(ctx, m, key, creds, m.httpForKey(key))
}

/*
wrap adds retries, the per-model breakers and timeout enforcement.
*/
func (m *LLMManager) wrap(key models.AiProviderKey, drv contracts.LLM) contracts.LLM {
	prefix := key.ID.String() + ":"
	return llmretry.Wrap(drv, llmretry.Options{
		Clock:         m.clock,
		MaxAttempts:   m.config.Retry.MaxAttempts,
		MaxRetryAfter: m.config.Retry.MaxRetryAfter,
		BreakerFor:    func(model string) *llmretry.Breaker { return m.breakers.For(prefix + model) },
	})
}

/*
httpForKey returns the egress-guarded client the driver uses.
*/
func (m *LLMManager) httpForKey(_ models.AiProviderKey) *httpclient.Client {
	return m.http
}

/*
ManagedAvailable reports whether the install has an upstream key for
the built-in provider, which is then offered to every workspace.
*/
func (m *LLMManager) ManagedAvailable() bool {
	return len(m.ManagedModels()) > 0
}

/*
ManagedModels lists the built-in provider's models whose family has
an upstream key configured.
*/
func (m *LLMManager) ManagedModels() []aimodels.Entry {
	return aimodels.ManagedFor(m.managedFamily)
}

/*
managedFamily reports whether a family has an upstream key.
*/
func (m *LLMManager) managedFamily(family enums.AIModelFamily) bool {
	cfg := m.config.Managed
	switch family {
	case enums.AIModelFamilyAnthropic:
		return cfg.Anthropic.APIKey != ""
	case enums.AIModelFamilyOpenAI:
		return cfg.OpenAI.APIKey != ""
	case enums.AIModelFamilyOpenWeight:
		return cfg.OpenWeight.Configured()
	}
	return false
}
