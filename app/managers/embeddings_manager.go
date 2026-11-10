/*
|--------------------------------------------------------------------------
| Embeddings Manager
|--------------------------------------------------------------------------
|
| One place builds a contracts.Embeddings: from a workspace's provider
| key, whose sealed credentials it opens in memory and never keeps, or
| from the install's own built-in embedder when no key is named. Drivers
| are registered per provider family, and built drivers are cached by
| key id and updated_at, so a rotated key never keeps its old driver.
|
*/

package managers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/dimmi"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openai"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openaicompat"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
ErrNoEmbedder is returned for a key whose provider cannot embed, or for
the built-in embedder on an install that configured none.
*/
var ErrNoEmbedder = errors.New("no embeddings driver")

/*
EmbeddingsFactory builds a driver for a key row, nil for the built-in
embedder, with the decrypted credentials and the HTTP client.
*/
type EmbeddingsFactory func(
	ctx context.Context, m *EmbeddingsManager, key *models.AiProviderKey,
	creds Credentials, httpClient *httpclient.Client,
) (contracts.Embeddings, error)

/*
EmbeddingsManager resolves and caches embeddings drivers.
*/
type EmbeddingsManager struct {
	keys      *encryption.Keys
	http      *httpclient.Client
	config    config.AIConfig
	clock     clock.Clock
	factories map[string]EmbeddingsFactory
	mu        sync.Mutex
	cache     map[string]contracts.Embeddings
}

/*
EmbeddingsDeps are what the manager needs from the application.
*/
type EmbeddingsDeps struct {
	Keys   *encryption.Keys
	HTTP   *httpclient.Client
	Config config.AIConfig
	Clock  clock.Clock
}

/*
NewEmbeddingsManager builds the manager with the built-in, OpenAI and
OpenAI-compatible drivers registered.
*/
func NewEmbeddingsManager(deps EmbeddingsDeps) *EmbeddingsManager {
	m := &EmbeddingsManager{
		keys: deps.Keys, http: deps.HTTP, config: deps.Config, clock: deps.Clock,
		factories: map[string]EmbeddingsFactory{}, cache: map[string]contracts.Embeddings{},
	}
	if m.clock == nil {
		m.clock = clock.System()
	}
	m.registerBuiltins()
	return m
}

/*
Extend registers a provider family's driver factory, replacing any
before it, and forgets the drivers already built.
*/
func (m *EmbeddingsManager) Extend(provider string, factory EmbeddingsFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factories[provider] = factory
	m.cache = map[string]contracts.Embeddings{}
}

/*
Builtin reports whether the install configured the built-in embedder.
*/
func (m *EmbeddingsManager) Builtin() bool {
	return m.config.Managed.Embeddings.APIKey != ""
}

/*
BuiltinModelKey is the built-in embedder's model key, "" when the
install configured none.
*/
func (m *EmbeddingsManager) BuiltinModelKey() string {
	if !m.Builtin() {
		return ""
	}
	e := m.config.Managed.Embeddings
	return dimmi.Provider + ":" + e.Model + "@" + strconv.Itoa(e.Dims)
}

/*
Embeds reports whether a key's provider family has a driver.
*/
func (m *EmbeddingsManager) Embeds(provider string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.factories[provider]
	return ok && provider != dimmi.Provider
}

/*
ForKey returns the driver for a provider key, or the built-in embedder
when the key is nil.
*/
func (m *EmbeddingsManager) ForKey(ctx context.Context, key *models.AiProviderKey) (contracts.Embeddings, error) {
	provider, cacheKey := dimmi.Provider, dimmi.Provider
	if key != nil {
		provider, cacheKey = key.Provider, key.ID.String()+":"+key.UpdatedAt.String()
	}
	m.mu.Lock()
	factory, ok := m.factories[provider]
	hit, cached := m.cache[cacheKey]
	m.mu.Unlock()
	if cached {
		return hit, nil
	}
	if !ok {
		return nil, fmt.Errorf("%w for provider %q", ErrNoEmbedder, provider)
	}
	var creds Credentials
	if key != nil {
		var err error
		if creds, err = OpenCredentials(ctx, m.keys, *key); err != nil {
			return nil, err
		}
	}
	drv, err := factory(ctx, m, key, creds, m.http)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	if len(m.cache) >= cacheLimit {
		m.cache = map[string]contracts.Embeddings{}
	}
	m.cache[cacheKey] = drv
	m.mu.Unlock()
	return drv, nil
}

/*
registerBuiltins registers the launch set: the built-in embedder,
OpenAI and any OpenAI-compatible server.
*/
func (m *EmbeddingsManager) registerBuiltins() {
	m.factories[dimmi.Provider] = func(
		_ context.Context, m *EmbeddingsManager, _ *models.AiProviderKey, _ Credentials, hc *httpclient.Client,
	) (contracts.Embeddings, error) {
		e := m.config.Managed.Embeddings
		if e.APIKey == "" {
			return nil, fmt.Errorf("%w: built-in embedder not configured", ErrNoEmbedder)
		}
		return dimmi.New(dimmi.Options{
			BaseURL: e.BaseURL, APIKey: e.APIKey, Model: e.Model, Dims: e.Dims, HTTP: hc, Clock: m.clock,
		}), nil
	}
	m.factories[string(enums.AIProviderOpenAI)] = func(
		_ context.Context, m *EmbeddingsManager, key *models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.Embeddings, error) {
		return openai.New(openai.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey, Headers: creds.ExtraHeaders, HTTP: hc, Clock: m.clock,
		}), nil
	}
	m.factories[string(enums.AIProviderOpenAICompatible)] = func(
		_ context.Context, m *EmbeddingsManager, key *models.AiProviderKey, creds Credentials, hc *httpclient.Client,
	) (contracts.Embeddings, error) {
		return openaicompat.New(openaicompat.Options{
			Provider: string(enums.AIProviderOpenAICompatible), BaseURL: str(key.BaseUrl), APIKey: creds.APIKey,
			Headers: creds.ExtraHeaders, HTTP: hc, Clock: m.clock,
		}), nil
	}
}
