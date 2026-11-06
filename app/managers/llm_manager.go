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
	"net/url"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/anthropic"
	"github.com/dimmihq/dimmi/app/drivers/llm/chatgpt"
	"github.com/dimmihq/dimmi/app/drivers/llm/engy"
	"github.com/dimmihq/dimmi/app/drivers/llm/managed"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
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
Credentials is a connection's opened secret shape.
*/
type Credentials struct {
	APIKey       string            `json:"api_key,omitempty"`
	AccessToken  string            `json:"access_token,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	AccountID    string            `json:"account_id,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at,omitzero"`
	Scopes       []string          `json:"scopes,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}

/*
LLMManager resolves and caches drivers. The cache key is the key id
with its updated_at, so a rotation or re-seal retires the old driver.
*/
type LLMManager struct {
	keys      *encryption.Keys
	db        *database.DB
	http      *httpclient.Client
	config    config.AIConfig
	clock     clock.Clock
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
	m := &LLMManager{
		keys: deps.Keys, http: deps.HTTP, config: deps.Config, clock: deps.Clock, db: deps.DB,
		factories: make(map[enums.AIProvider]DriverFactory), cache: map[string]cachedDriver{},
	}
	m.registerBuiltins()
	return m
}

/*
registerBuiltins registers the drivers of the launch set.
*/
func (m *LLMManager) registerBuiltins() {
	m.registerKeyDriven()
	m.registerSources()
}

/*
registerKeyDriven registers the providers that run on key rows.
*/
func (m *LLMManager) registerKeyDriven() {
	m.Extend(enums.AIProviderAnthropic, func(
		_ context.Context, _ *LLMManager, key models.AiProviderKey,
		creds Credentials, httpClient *httpclient.Client,
	) (contracts.LLM, error) {
		return anthropic.New(anthropic.Options{
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey,
			Version: configString(key.Config, "anthropic_version"), HTTP: httpClient.StdClient(),
		}), nil
	})

	m.Extend(enums.AIProviderOpenAI, func(
		_ context.Context, _ *LLMManager, key models.AiProviderKey,
		creds Credentials, httpClient *httpclient.Client,
	) (contracts.LLM, error) {
		return openai.New(openai.Options{
			Clock:   m.clock,
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey,
			Headers: creds.ExtraHeaders, HTTP: httpClient.StdClient(),
		}), nil
	})

	m.Extend(enums.AIProviderEngy, func(
		_ context.Context, _ *LLMManager, _ models.AiProviderKey,
		creds Credentials, httpClient *httpclient.Client,
	) (contracts.LLM, error) {
		return engy.New(creds.APIKey, httpClient.StdClient(), m.clock), nil
	})

	m.Extend(enums.AIProviderOpenAICompatible, func(
		_ context.Context, _ *LLMManager, key models.AiProviderKey,
		creds Credentials, httpClient *httpclient.Client,
	) (contracts.LLM, error) {
		return openai.New(openai.Options{
			Clock:   m.clock,
			BaseURL: str(key.BaseUrl), APIKey: creds.APIKey,
			Headers: creds.ExtraHeaders, HTTP: httpClient.StdClient(),
		}), nil
	})
}

/*
registerSources registers the providers that hold no user key.
*/
func (m *LLMManager) registerSources() {
	m.Extend(enums.AIProviderChatGPT, func(
		_ context.Context, m *LLMManager, key models.AiProviderKey,
		creds Credentials, httpClient *httpclient.Client,
	) (contracts.LLM, error) {
		source := &oauthSource{m: m, key: key, creds: creds}
		return chatgpt.New(m.config.ChatGPT.APIBaseURL, httpClient.StdClient(), source, m.clock), nil
	})

	m.Extend(enums.AIProviderDimmi, func(
		_ context.Context, m *LLMManager, _ models.AiProviderKey,
		_ Credentials, httpClient *httpclient.Client,
	) (contracts.LLM, error) {
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
	m.cache[cacheKey] = cachedDriver{llm: drv, at: m.clock.Now()}
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

	creds, err := encryption.Open[Credentials](ctx, m.keys, key.CredentialsEncrypted, []byte(key.ID.String()))
	if err != nil {
		return Credentials{}, fmt.Errorf("open provider credentials: %w", err)
	}

	return creds, nil
}

/*
httpForKey returns the egress-guarded client the driver uses.
*/
func (m *LLMManager) httpForKey(_ models.AiProviderKey) *httpclient.Client {
	return m.http
}

/*
managedDriver builds the multiplexer from the instance's managed
credentials.
*/
func (m *LLMManager) managedDriver(httpClient *httpclient.Client) (contracts.LLM, error) {
	cfg := m.config.Managed
	families := make(map[enums.AIModelFamily]contracts.LLM)

	if cfg.Anthropic.APIKey != "" {
		families[enums.AIModelFamilyAnthropic] = anthropic.New(anthropic.Options{
			BaseURL: cfg.Anthropic.BaseURL, APIKey: cfg.Anthropic.APIKey, HTTP: httpClient.StdClient(),
		})
	}
	if cfg.OpenAI.APIKey != "" {
		families[enums.AIModelFamilyOpenAI] = openai.New(openai.Options{
			Clock:   m.clock,
			BaseURL: cfg.OpenAI.BaseURL, APIKey: cfg.OpenAI.APIKey, HTTP: httpClient.StdClient(),
		})
	}
	if cfg.OpenWeight.APIKey != "" {
		families[enums.AIModelFamilyOpenWeight] = engy.New(cfg.OpenWeight.APIKey, httpClient.StdClient(), m.clock)
	}

	if len(families) == 0 {
		return nil, fmt.Errorf("managed provider not configured")
	}

	modelList := []contracts.LLMModel{}
	for _, e := range aimodels.Managed() {
		if _, ok := families[e.Family]; ok {
			modelList = append(modelList, contracts.LLMModel{
				ID: e.ID, DisplayName: e.DisplayName, ContextTokens: e.ContextTokens,
			})
		}
	}

	return managed.New(families, modelList), nil
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
	if s.creds.AccessToken != "" && s.m.clock.Now().Before(s.creds.ExpiresAt.Add(-2*time.Minute)) {
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
	if s.m.db == nil || s.creds.RefreshToken == "" {
		return "", fmt.Errorf("chatgpt refresh unavailable")
	}
	cfg := s.m.config.ChatGPT

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {cfg.ClientID},
		"refresh_token": {s.creds.RefreshToken},
	}
	resp, err := s.m.http.New().
		WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Post(ctx, cfg.TokenURL, form.Encode())
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(resp.Body, &tokens); err != nil || tokens.AccessToken == "" {
		return "", fmt.Errorf("bad refresh answer")
	}
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = s.creds.RefreshToken
	}

	s.creds.AccessToken = tokens.AccessToken
	s.creds.RefreshToken = tokens.RefreshToken
	s.creds.ExpiresAt = s.m.clock.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	sealed, err := encryption.Seal(ctx, s.m.keys,
		encryption.Slot{Scope: encryption.Scope{Kind: "workspace", ID: s.key.WorkspaceID.String()}},
		s.creds)
	if err != nil {
		return "", fmt.Errorf("seal refreshed credentials: %w", err)
	}

	if _, err := models.New(s.m.db).UpdateProviderKey(ctx, models.UpdateProviderKeyParams{
		WorkspaceID: s.key.WorkspaceID, ID: s.key.ID, Name: s.key.Name,
		BaseUrl: s.key.BaseUrl, Region: s.key.Region, AuthType: s.key.AuthType,
		CredentialsEncrypted: sealed, KeyHint: s.key.KeyHint, Config: s.key.Config,
		Now: s.m.clock.Now(),
	}); err != nil {
		return "", fmt.Errorf("store refreshed credentials: %w", err)
	}

	return s.creds.AccessToken, nil
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
