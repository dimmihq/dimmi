package socialite_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/oidc"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
Redirect URLs the test providers list.
*/
const (
	callbackURL  = "https://app.example.com/auth/callback"
	alternateURL = "https://eu.example.com/auth/callback"
)

func newStore(t *testing.T) *cache.Store {
	t.Helper()

	store, err := cache.Open(testkit.NewValkey(t), cache.Options{Keyspace: "cache"})
	require.NoError(t, err)
	t.Cleanup(store.Close)

	return store
}

/*
newManager returns a manager whose providers are faked, keeping its
attempts in a store of the test's own.
*/
func newManager(t *testing.T) (*socialite.Manager, *socialite.Fake) {
	t.Helper()

	m := socialite.NewManager(socialite.Config{Providers: map[string]config.Section{
		"google": {"driver": "google", "redirect_url": callbackURL, "redirect_urls": []any{alternateURL}},
		"github": {"driver": "github", "redirect_url": callbackURL},
		"bare":   {"driver": "google"},
		"odd":    {"driver": "google", "redirect_url": map[string]any{"not": "a url"}},
	}, AllowFakes: true}, newStore(t))
	fake := m.Fake()

	return m, fake
}

/*
startSignIn redirects to the provider and approves as the user, and
returns the state and callback query.
*/
func startSignIn(t *testing.T, m *socialite.Manager, fake *socialite.Fake, user socialite.User) (string, url.Values) {
	t.Helper()

	authURL, state, err := m.Redirect(t.Context(), "google", "")
	require.NoError(t, err)

	callback, err := fake.Approve(authURL, user)
	require.NoError(t, err)

	parsed, err := url.Parse(callback)
	require.NoError(t, err)

	return state, parsed.Query()
}

func TestSignIn(t *testing.T) {
	t.Parallel()

	ada := socialite.User{ID: "108", Email: "ada@example.com", EmailVerified: true, Name: "Ada"}

	t.Run("signs a user in from redirect to callback", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		state, query := startSignIn(t, m, fake, ada)

		user, err := m.Callback(t.Context(), "google", state, query)

		require.NoError(t, err)
		assert.Equal(t, "google", user.Provider)
		assert.Equal(t, "108", user.ID)
		assert.Equal(t, "ada@example.com", user.Email)
		assert.True(t, user.EmailVerified)
		assert.NotEmpty(t, user.Token.AccessToken)
	})

	t.Run("sends users back to another listed redirect url", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)

		authURL, state, err := m.Redirect(t.Context(), "google", alternateURL)
		require.NoError(t, err)
		callback, err := fake.Approve(authURL, ada)
		require.NoError(t, err)
		parsed, err := url.Parse(callback)
		require.NoError(t, err)

		assert.Equal(t, "eu.example.com", parsed.Host)
		_, err = m.Callback(t.Context(), "google", state, parsed.Query())
		require.NoError(t, err)
	})

	t.Run("finishes an attempt only once", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		state, query := startSignIn(t, m, fake, ada)

		_, err := m.Callback(t.Context(), "google", state, query)
		require.NoError(t, err)

		_, err = m.Callback(t.Context(), "google", state, query)
		require.ErrorIs(t, err, socialite.ErrInvalidState)
	})
}

func TestRedirect(t *testing.T) {
	t.Parallel()

	t.Run("refuses a redirect url the provider does not list", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		_, _, err := m.Redirect(t.Context(), "google", "https://evil.example.com/callback")
		require.ErrorIs(t, err, socialite.ErrRedirectNotAllowed)

		_, _, err = m.Redirect(t.Context(), "bare", "")
		require.ErrorIs(t, err, socialite.ErrRedirectNotAllowed)

		_, _, err = m.Redirect(t.Context(), "odd", "")
		require.ErrorContains(t, err, "provider odd")
	})

	t.Run("refuses an unknown provider", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		_, _, err := m.Redirect(t.Context(), "myspace", "")
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})

	t.Run("fails when the provider cannot be reached", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
		require.NoError(t, err)
		m := socialite.NewManager(socialite.Config{Providers: map[string]config.Section{
			"sso": {"driver": "oidc", "issuer": "http://127.0.0.1:1", "client_id": "c", "redirect_url": callbackURL},
		}}, newStore(t))
		m.Extend("oidc", oidc.Factory(client, clock.System()))

		_, _, err = m.Redirect(t.Context(), "sso", "")
		require.ErrorIs(t, err, socialite.ErrProvider)
	})

	t.Run("fails when the store cannot keep the attempt", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, _, err := m.Redirect(ctx, "google", "")
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestCallback(t *testing.T) {
	t.Parallel()

	ada := socialite.User{ID: "108", Email: "ada@example.com"}

	t.Run("refuses a callback in another browser", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		_, query := startSignIn(t, m, fake, ada)

		_, err := m.Callback(t.Context(), "google", "someone-else", query)
		require.ErrorIs(t, err, socialite.ErrInvalidState)

		_, err = m.Callback(t.Context(), "google", "", url.Values{})
		require.ErrorIs(t, err, socialite.ErrInvalidState)
	})

	t.Run("refuses a state it never issued", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		_, err := m.Callback(t.Context(), "google", "forged", url.Values{"state": {"forged"}, "code": {"c"}})
		require.ErrorIs(t, err, socialite.ErrInvalidState)
	})

	t.Run("refuses an attempt started with another provider", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		state, query := startSignIn(t, m, fake, ada)

		_, err := m.Callback(t.Context(), "github", state, query)
		require.ErrorIs(t, err, socialite.ErrInvalidState)
	})

	t.Run("refuses an unknown provider", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		_, err := m.Callback(t.Context(), "myspace", "s", url.Values{"state": {"s"}})
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})

	t.Run("reports errors the provider sends back", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)

		state, query := startSignIn(t, m, fake, ada)
		query.Set("error", "access_denied")
		_, err := m.Callback(t.Context(), "google", state, query)
		require.ErrorIs(t, err, socialite.ErrDenied)

		state, query = startSignIn(t, m, fake, ada)
		query.Set("error", "temporarily_unavailable")
		_, err = m.Callback(t.Context(), "google", state, query)
		require.ErrorIs(t, err, socialite.ErrProvider)
		require.ErrorContains(t, err, "temporarily_unavailable")
	})

	t.Run("refuses a missing or forged code", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)

		state, query := startSignIn(t, m, fake, ada)
		query.Del("code")
		_, err := m.Callback(t.Context(), "google", state, query)
		require.ErrorIs(t, err, socialite.ErrExchange)

		state, query = startSignIn(t, m, fake, ada)
		query.Set("code", "forged")
		_, err = m.Callback(t.Context(), "google", state, query)
		require.ErrorIs(t, err, socialite.ErrExchange)
	})

	t.Run("fails when the store cannot be read", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		state, query := startSignIn(t, m, fake, ada)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := m.Callback(ctx, "google", state, query)
		require.ErrorIs(t, err, context.Canceled)
	})
}
