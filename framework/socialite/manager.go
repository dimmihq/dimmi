/*
|--------------------------------------------------------------------------
| Provider Manager
|--------------------------------------------------------------------------
|
| Providers are declared in configuration by name, each naming its
| driver and the redirect URLs it may send users back to. Drivers plug
| in through Extend.
|
|   socialite:
|     providers:
|       google:
|         driver: google
|         client_id: ...
|         redirect_url: https://app.example.com/auth/google/callback
|         redirect_urls: [https://eu.example.com/auth/google/callback]
|
| The secret half of an attempt, its verifier and nonce, never leaves the
| server: it waits in the cache under the state until the callback
| claims it, once, or it expires. The application keeps the state in
| the browser that started the attempt and hands it back on the
| callback, so a callback link sent to another browser fails.
|
*/

package socialite

import (
	"cmp"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
Defaults and bounds: how long an attempt may take, how long a callback
may spend talking to the provider, and how many characters the state and
nonce have, about 256 random bits.
*/
const (
	DefaultStateTTL = 10 * time.Minute
	exchangeTimeout = 10 * time.Second
	tokenLength     = 43
)

/*
Config lists the named providers and how long an attempt stays open.
AllowFakes, set in code and only when the application runs its tests,
lets Fake replace the providers.
*/
type Config struct {
	StateTTL   time.Duration             `default:"10m" koanf:"state_ttl"`
	Providers  map[string]config.Section `koanf:"providers"`
	AllowFakes bool                      `koanf:"-"`
}

/*
Manager resolves named providers and runs sign-in attempts against them.
*/
type Manager struct {
	providers *manager.Manager[Provider]
	sections  map[string]config.Section
	store     *cache.Store
	ttl       time.Duration
	fakes     bool
}

/*
NewManager returns a manager for the providers in the configuration,
keeping attempts in the store.
*/
func NewManager(cfg Config, store *cache.Store) *Manager {
	return &Manager{
		providers: manager.New[Provider]("provider", cfg.Providers, ""),
		sections:  cfg.Providers,
		store:     store,
		ttl:       cmp.Or(cfg.StateTTL, DefaultStateTTL),
		fakes:     cfg.AllowFakes,
	}
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[Provider]) {
	m.providers.Extend(driver, factory)
}

/*
Driver returns the named provider.
*/
func (m *Manager) Driver(name string) (Provider, error) {
	return m.providers.Instance(name)
}

/*
Build returns a provider from settings kept as data rather than
configuration, such as a company's own issuer, through the driver the
section names. It is built fresh on every call and takes no part in
Redirect and Callback, so its caller keeps the attempt.
*/
func (m *Manager) Build(name string, section config.Section) (Provider, error) {
	return m.providers.Build(name, section)
}

/*
Redirect starts an attempt with the named provider and returns the URL
to send the browser to and the state to keep in that browser. An empty
redirect URL means the provider's configured one; any other must be
listed in its configuration.
*/
func (m *Manager) Redirect(ctx context.Context, provider, redirectURL string) (string, string, error) {
	driver, err := m.Driver(provider)
	if err != nil {
		return "", "", err
	}

	redirectURL, err = m.allowedRedirect(provider, redirectURL)
	if err != nil {
		return "", "", err
	}

	attempt := Attempt{
		State:       str.Random(tokenLength),
		Verifier:    oauth2.GenerateVerifier(),
		Nonce:       str.Random(tokenLength),
		RedirectURL: redirectURL,
	}

	authURL, err := driver.AuthURL(ctx, attempt)
	if err != nil {
		return "", "", fmt.Errorf("redirect to %s: %w", provider, err)
	}

	if err := m.remember(ctx, provider, attempt); err != nil {
		return "", "", err
	}

	return authURL, attempt.State, nil
}

/*
Callback finishes an attempt from the query the provider sent the
browser back with. The state is the one Redirect returned, kept by the
browser; the query must carry the same one, and each state finishes at
most one attempt. The code is then traded for the user.
*/
func (m *Manager) Callback(ctx context.Context, provider, state string, query url.Values) (User, error) {
	driver, err := m.Driver(provider)
	if err != nil {
		return User{}, err
	}

	attempt, err := m.claim(ctx, provider, state, query.Get("state"))
	if err != nil {
		return User{}, err
	}

	if err := callbackError(query); err != nil {
		return User{}, err
	}

	code := query.Get("code")
	if code == "" {
		return User{}, fmt.Errorf("%w: callback has no code", ErrExchange)
	}

	ctx, cancel := context.WithTimeout(ctx, exchangeTimeout)
	defer cancel()

	token, err := driver.Exchange(ctx, code, attempt)
	if err != nil {
		return User{}, fmt.Errorf("exchange %s code: %w", provider, err)
	}

	user, err := driver.User(ctx, token)
	if err != nil {
		return User{}, fmt.Errorf("read %s user: %w", provider, err)
	}

	user.Token = token

	return user, nil
}

/*
redirectOptions are the options every provider section shares.
*/
type redirectOptions struct {
	RedirectURL  string   `koanf:"redirect_url"`
	RedirectURLs []string `koanf:"redirect_urls"`
}

func (m *Manager) allowedRedirect(provider, requested string) (string, error) {
	var opts redirectOptions
	if err := m.sections[provider].Decode(&opts); err != nil {
		return "", fmt.Errorf("provider %s: %w", provider, err)
	}

	requested = cmp.Or(requested, opts.RedirectURL)
	if requested == "" || (requested != opts.RedirectURL && !slices.Contains(opts.RedirectURLs, requested)) {
		return "", fmt.Errorf("%w: %q for %s", ErrRedirectNotAllowed, requested, provider)
	}

	return requested, nil
}

/*
attemptRecord is what the cache holds for an open attempt.
*/
type attemptRecord struct {
	Provider    string `json:"provider"`
	Verifier    string `json:"verifier"`
	Nonce       string `json:"nonce"`
	RedirectURL string `json:"redirect_url"`
}

func (m *Manager) remember(ctx context.Context, provider string, attempt Attempt) error {
	record, err := json.Marshal(attemptRecord{
		Provider:    provider,
		Verifier:    attempt.Verifier,
		Nonce:       attempt.Nonce,
		RedirectURL: attempt.RedirectURL,
	})
	if err != nil {
		return fmt.Errorf("encode attempt: %w", err)
	}

	added, err := m.store.Add(ctx, attemptKey(attempt.State), record, m.ttl)
	if err != nil {
		return fmt.Errorf("store attempt: %w", err)
	}

	if !added {
		return fmt.Errorf("store attempt: %w: state already in use", ErrInvalidState)
	}

	return nil
}

/*
claim returns the open attempt for the state and closes it. The marker
added under the state lets exactly one of two racing callbacks through.
*/
func (m *Manager) claim(ctx context.Context, provider, kept, returned string) (Attempt, error) {
	if kept == "" || subtle.ConstantTimeCompare([]byte(kept), []byte(returned)) != 1 {
		return Attempt{}, fmt.Errorf("%w: state does not match the browser", ErrInvalidState)
	}

	raw, err := m.store.Get(ctx, attemptKey(kept))
	if errors.Is(err, cache.ErrMiss) {
		return Attempt{}, fmt.Errorf("%w: unknown or expired", ErrInvalidState)
	}

	if err != nil {
		return Attempt{}, fmt.Errorf("load attempt: %w", err)
	}

	claimed, err := m.store.Add(ctx, claimKey(kept), []byte{1}, m.ttl)
	if err != nil {
		return Attempt{}, fmt.Errorf("claim attempt: %w", err)
	}

	var record attemptRecord
	if !claimed || json.Unmarshal(raw, &record) != nil || record.Provider != provider {
		return Attempt{}, fmt.Errorf("%w: already used or not for %s", ErrInvalidState, provider)
	}

	return Attempt{State: kept, Verifier: record.Verifier, Nonce: record.Nonce, RedirectURL: record.RedirectURL}, nil
}

/*
callbackError maps the error a provider reports on the callback.
*/
func callbackError(query url.Values) error {
	code := query.Get("error")
	if code == "" {
		return nil
	}

	if code == "access_denied" {
		return ErrDenied
	}

	return fmt.Errorf("%w: %s", ErrProvider, code)
}

func attemptKey(state string) string {
	return "socialite:attempt:" + state
}

func claimKey(state string) string {
	return "socialite:claimed:" + state
}
