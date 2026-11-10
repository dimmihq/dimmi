/*
|--------------------------------------------------------------------------
| Provider Manager
|--------------------------------------------------------------------------
|
| Providers are declared in configuration by name, each naming its
| driver and where the provider sends the browser back. Drivers plug in
| through Extend.
|
|   oauth_client:
|     state_ttl: 10m
|     providers:
|       chat:
|         driver: generic
|         client_id: ...
|         redirect_url: https://app.example.com/connections/chat/callback
|
| The secret half of an attempt, its verifier, the session it belongs to
| and what the caller asked to get back, never leaves the server: it
| waits in the cache under the state until the callback claims it, once,
| or it expires.
|
*/

package oauthclient

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
Defaults and bounds: how long an attempt may take, how long a call to
the provider may take, and how many characters a state has, about 256
random bits.
*/
const (
	DefaultStateTTL = 10 * time.Minute
	providerTimeout = 10 * time.Second
	stateLength     = 43
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
Payload is what the caller attached to an attempt, handed back on its
callback.
*/
type Payload map[string]string

/*
Manager resolves named providers and runs connection attempts against
them.
*/
type Manager struct {
	providers *manager.Manager[Driver]
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
		providers: manager.New[Driver]("provider", cfg.Providers, ""),
		sections:  cfg.Providers,
		store:     store,
		ttl:       cmp.Or(cfg.StateTTL, DefaultStateTTL),
		fakes:     cfg.AllowFakes,
	}
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[Driver]) {
	m.providers.Extend(driver, factory)
}

/*
Driver returns the named provider.
*/
func (m *Manager) Driver(name string) (Driver, error) {
	return m.providers.Instance(name)
}

/*
Has reports whether the configuration declares the named provider.
*/
func (m *Manager) Has(name string) bool {
	return slices.Contains(m.providers.Names(), name)
}

/*
Redirect starts an attempt with the named provider and returns the URL
to send the browser to and the attempt's state. The request must name
the browser session it belongs to.
*/
func (m *Manager) Redirect(ctx context.Context, provider string, req Request) (string, string, error) {
	driver, err := m.Driver(provider)
	if err != nil {
		return "", "", err
	}

	if req.Binding == "" {
		return "", "", fmt.Errorf("%w: the attempt names no session", ErrInvalidState)
	}

	redirectURL := cmp.Or(req.RedirectURL, m.sections[provider].String("redirect_url"))
	if redirectURL == "" {
		return "", "", fmt.Errorf("%w: %s has no redirect_url", ErrMisconfigured, provider)
	}

	attempt := Attempt{
		State: str.Random(stateLength), Verifier: oauth2.GenerateVerifier(), RedirectURL: redirectURL,
		Scopes: req.Scopes, UserScopes: req.UserScopes, Extra: req.Extra,
	}

	authURL, err := driver.AuthURL(ctx, attempt)
	if err != nil {
		return "", "", fmt.Errorf("redirect to %s: %w", provider, err)
	}

	if err := m.remember(ctx, provider, attempt, req); err != nil {
		return "", "", err
	}

	return authURL, attempt.State, nil
}

/*
Callback finishes an attempt from the query the provider sent the
browser back with, in the session the binding names. Each state
finishes at most one attempt. The payload comes back whenever the
attempt was found, also when the person declined, so the caller can
still send them back where they came from.
*/
func (m *Manager) Callback(ctx context.Context, provider, binding string, query url.Values) (Grant, Payload, error) {
	driver, err := m.Driver(provider)
	if err != nil {
		return Grant{}, nil, err
	}

	attempt, payload, err := m.claim(ctx, provider, binding, query.Get("state"))
	if err != nil {
		return Grant{}, nil, err
	}

	if err := callbackError(query); err != nil {
		return Grant{}, payload, err
	}

	code := query.Get("code")
	if code == "" {
		return Grant{}, payload, fmt.Errorf("%w: callback has no code", ErrExchange)
	}

	ctx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	grant, err := driver.Exchange(ctx, code, attempt)
	if err != nil {
		return Grant{}, payload, fmt.Errorf("exchange %s code: %w", provider, err)
	}

	return grant, payload, nil
}

/*
Refresh trades a refresh token for a fresh grant from the named
provider.
*/
func (m *Manager) Refresh(ctx context.Context, provider, refresh string) (Grant, error) {
	driver, err := m.Driver(provider)
	if err != nil {
		return Grant{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	grant, err := driver.Refresh(ctx, refresh)
	if err != nil {
		return Grant{}, fmt.Errorf("refresh %s token: %w", provider, err)
	}

	return grant, nil
}

/*
Revoke gives a token back to the named provider.
*/
func (m *Manager) Revoke(ctx context.Context, provider, token string) error {
	driver, err := m.Driver(provider)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	if err := driver.Revoke(ctx, token); err != nil {
		return fmt.Errorf("revoke %s token: %w", provider, err)
	}

	return nil
}

/*
attemptRecord is what the cache holds for an open attempt.
*/
type attemptRecord struct {
	Provider    string            `json:"provider"`
	Binding     string            `json:"binding"`
	Verifier    string            `json:"verifier"`
	RedirectURL string            `json:"redirect_url"`
	Scopes      []string          `json:"scopes"`
	UserScopes  []string          `json:"user_scopes"`
	Extra       map[string]string `json:"extra"`
	Payload     Payload           `json:"payload"`
}

func (m *Manager) remember(ctx context.Context, provider string, attempt Attempt, req Request) error {
	record, err := json.Marshal(attemptRecord{
		Provider: provider, Binding: req.Binding, Verifier: attempt.Verifier, RedirectURL: attempt.RedirectURL,
		Scopes: attempt.Scopes, UserScopes: attempt.UserScopes, Extra: attempt.Extra, Payload: req.Payload,
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
claim returns the open attempt for the state and closes it. A session
other than the one that started the attempt is refused before the
attempt is spent, so a stray browser cannot spend it either. The marker
added under the state lets exactly one of two racing callbacks through.
*/
func (m *Manager) claim(ctx context.Context, provider, binding, state string) (Attempt, Payload, error) {
	if state == "" || binding == "" {
		return Attempt{}, nil, fmt.Errorf("%w: missing state or session", ErrInvalidState)
	}

	raw, err := m.store.Get(ctx, attemptKey(state))
	if errors.Is(err, cache.ErrMiss) {
		return Attempt{}, nil, fmt.Errorf("%w: unknown or expired", ErrInvalidState)
	}

	if err != nil {
		return Attempt{}, nil, fmt.Errorf("load attempt: %w", err)
	}

	var record attemptRecord
	if json.Unmarshal(raw, &record) != nil || record.Provider != provider ||
		subtle.ConstantTimeCompare([]byte(record.Binding), []byte(binding)) != 1 {
		return Attempt{}, nil, fmt.Errorf("%w: not started by this session for %s", ErrInvalidState, provider)
	}

	claimed, err := m.store.Add(ctx, claimKey(state), []byte{1}, m.ttl)
	if err != nil {
		return Attempt{}, nil, fmt.Errorf("claim attempt: %w", err)
	}

	if !claimed {
		return Attempt{}, nil, fmt.Errorf("%w: already used", ErrInvalidState)
	}

	return Attempt{
		State: state, Verifier: record.Verifier, RedirectURL: record.RedirectURL,
		Scopes: record.Scopes, UserScopes: record.UserScopes, Extra: record.Extra,
	}, record.Payload, nil
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
	return "oauthclient:attempt:" + state
}

func claimKey(state string) string {
	return "oauthclient:claimed:" + state
}
