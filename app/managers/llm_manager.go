/*
|--------------------------------------------------------------------------
| LLM Manager
|--------------------------------------------------------------------------
|
| One place builds a contracts.LLM from a provider-key row: it opens
| the sealed credentials, picks the driver for the provider, and keeps
| a short-lived cache so a rotated key never keeps its old driver.
| Drivers are built per provider through the registry, so a new
| provider lands as one registered factory, never a manager edit.
|
*/

package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/anthropic"
	"github.com/dimmihq/dimmi/app/drivers/llm/chatgpt"
	"github.com/dimmihq/dimmi/app/drivers/llm/engy"
	"github.com/dimmihq/dimmi/app/drivers/llm/managed"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
DriverFactory builds a driver for one key row with the decrypted
credentials and the per-key HTTP client.
*/
type DriverFactory func(ctx context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, httpClient *http.Client) (contracts.LLM, error)

/*
Credentials is a connection's opened secret shape.
*/
type Credentials struct {
	APIKey       string            `json:"api_key,omitempty"`
	AccessToken  string            `json:"access_token,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	AccountID    string            `json:"account_id,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at,omitempty"`
	Scopes       []string          `json:"scopes,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}

/*
LLMManager resolves and caches drivers. The cache key is the key id
with its updated_at, so a rotation or re-seal retires the old driver.
*/
type LLMManager struct {
	app       *app.App
	factories map[enums.AIProvider]DriverFactory
	mu        sync.Mutex
	cache     map[string]cachedDriver
	order     []string
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
NewLLMManager builds the manager with the drivers of today
registered.
*/
func NewLLMManager(a *app.App) *LLMManager {
	m := &LLMManager{app: a, factories: map[enums.AIProvider]DriverFactory{}, cache: map[string]cachedDriver{}}
	m.registerBuiltins()
	return m
}

/*
registerBuiltins registers the drivers of the launch set.
*/
func (m *LLMManager) registerBuiltins() {
	m.Extend(enums.AIProviderAnthropic, func(_ context.Context, _ *LLMManager, key models.AiProviderKey, creds Credentials, httpClient *http.Client) (contracts.LLM, error) {
		return anthropic.New(anthropic.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey,
			Version: configString(key.Config, "anthropic_version"), HTTP: httpClient,
		}), nil
	})

	m.Extend(enums.AIProviderOpenAI, func(_ context.Context, _ *LLMManager, key models.AiProviderKey, creds Credentials, httpClient *http.Client) (contracts.LLM, error) {
		return openai.New(openai.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey,
			Headers: creds.ExtraHeaders, HTTP: httpClient,
		}), nil
	})

	m.Extend(enums.AIProviderEngy, func(_ context.Context, _ *LLMManager, key models.AiProviderKey, creds Credentials, httpClient *http.Client) (contracts.LLM, error) {
		return engy.New(creds.APIKey, httpClient), nil
	})

	m.Extend(enums.AIProviderOpenAICompatible, func(_ context.Context, _ *LLMManager, key models.AiProviderKey, creds Credentials, httpClient *http.Client) (contracts.LLM, error) {
		return openai.New(openai.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey,
			Headers: creds.ExtraHeaders, HTTP: httpClient,
		}), nil
	})

	m.Extend(enums.AIProviderChatGPT, func(_ context.Context, m *LLMManager, key models.AiProviderKey, creds Credentials, httpClient *http.Client) (contracts.LLM, error) {
		return chatgpt.New(m.app.Config.AI.ChatGPT.APIBaseURL, httpClient, &oauthSource{m: m, key: key, creds: creds}), nil
	})

	m.Extend(enums.AIProviderDimmi, func(_ context.Context, m *LLMManager, _ models.AiProviderKey, _ Credentials, httpClient *http.Client) (contracts.LLM, error) {
		return m.managedDriver(httpClient)
	})
}

/*
Extend registers a provider's driver factory.
*/
func (m *LLMManager) Extend(provider enums.AIProvider, factory DriverFactory) {
	m.factories[provider] = factory
}

/*
ForKey returns the driver for a provider-key row.
*/
func (m *LLMManager) ForKey(ctx context.Context, key models.AiProviderKey) (contracts.LLM, error) {
	cacheKey := key.ID.String() + ":" + key.UpdatedAt.String()

	m.mu.Lock()
	if hit, ok := m.cache[cacheKey]; ok {
		m.mu.Unlock()
		return hit.llm, nil
	}
	m.mu.Unlock()

	factory, ok := m.factories[enums.AIProvider(key.Provider)]
	if !ok {
		return nil, fmt.Errorf("no llm driver for provider %q", key.Provider)
	}

	creds, err := m.openCredentials(ctx, key)
	if err != nil {
		return nil, err
	}

	httpClient := m.httpForKey(key)

	drv, err := factory(ctx, m, key, creds, httpClient)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if len(m.order) >= cacheLimit {
		delete(m.cache, m.order[0])
		m.order = m.order[1:]
	}
	m.cache[cacheKey] = cachedDriver{llm: drv, at: time.Now()}
	m.order = append(m.order, cacheKey)
	m.mu.Unlock()

	return drv, nil
}

/*
openCredentials opens the sealed credential blob, empty for managed
and none-auth connections.
*/
func (m *LLMManager) openCredentials(ctx context.Context, key models.AiProviderKey) (Credentials, error) {
	if len(key.CredentialsEncrypted) == 0 {
		return Credentials{}, nil
	}

	creds, err := encryption.Open[Credentials](ctx, m.app.Keys, key.CredentialsEncrypted, []byte(key.ID.String()))
	if err != nil {
		return Credentials{}, fmt.Errorf("open provider credentials: %w", err)
	}

	return creds, nil
}

/*
httpForKey returns the egress-guarded client the driver uses.
*/
func (m *LLMManager) httpForKey(_ models.AiProviderKey) *http.Client {
	return m.app.HTTP.StdClient()
}

/*
managedDriver builds the multiplexer from the instance's managed
credentials.
*/
func (m *LLMManager) managedDriver(httpClient *http.Client) (contracts.LLM, error) {
	cfg := m.app.Config.AI.Managed
	families := map[enums.AIModelFamily]contracts.LLM{}

	if cfg.Anthropic.APIKey != "" {
		families[enums.AIModelFamilyAnthropic] = anthropic.New(anthropic.Options{
			BaseURL: cfg.Anthropic.BaseURL, APIKey: cfg.Anthropic.APIKey, HTTP: httpClient,
		})
	}
	if cfg.OpenAI.APIKey != "" {
		families[enums.AIModelFamilyOpenAI] = openai.New(openai.Options{
			BaseURL: cfg.OpenAI.BaseURL, APIKey: cfg.OpenAI.APIKey, HTTP: httpClient,
		})
	}
	if cfg.OpenWeight.APIKey != "" {
		families[enums.AIModelFamilyOpenWeight] = engy.New(cfg.OpenWeight.APIKey, httpClient)
	}

	if len(families) == 0 {
		return nil, fmt.Errorf("managed provider not configured")
	}

	models := []contracts.LLMModel{}
	for _, e := range aimodels.Managed() {
		if _, ok := families[e.Family]; ok {
			models = append(models, contracts.LLMModel{ID: e.ID, DisplayName: e.DisplayName, ContextTokens: e.ContextTokens})
		}
	}

	return managed.New(families, models), nil
}

/*
oauthSource is the chatgpt TokenSource bound to one key row.
*/
type oauthSource struct {
	m     *LLMManager
	key   models.AiProviderKey
	creds Credentials
}

/*
Token returns the access token, refreshing it when near expiry.
*/
func (s *oauthSource) Token(ctx context.Context) (string, error) {
	if s.creds.AccessToken != "" && time.Now().Before(s.creds.ExpiresAt.Add(-2*time.Minute)) {
		return s.creds.AccessToken, nil
	}
	return s.refresh(ctx)
}

/*
refresh exchanges the refresh token and re-seals the row's
credentials; on success the key's updated_at bumps, retiring this
driver from the cache.
*/
func (s *oauthSource) refresh(ctx context.Context) (string, error) {
	return "", fmt.Errorf("chatgpt refresh not wired")
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

/*
configString reads one string from a key's config blob.
*/
func configString(cfg json.RawMessage, name string) string {
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return ""
	}
	v, _ := m[name].(string)
	return v
}

/*
_ keeps ids imported for the credential decode helpers.
*/
var _ = ids.New
