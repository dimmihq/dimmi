package oauthclient_test

import (
	"context"
	"maps"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
callbackURL is where the test providers send the browser back.
*/
const callbackURL = "https://app.example.com/connections/chat/callback"

func newStore(t *testing.T) *cache.Store {
	t.Helper()

	store, err := cache.Open(testkit.NewValkey(t), cache.Options{Keyspace: "cache"})
	require.NoError(t, err)
	t.Cleanup(store.Close)

	return store
}

/*
sections are the providers the tests configure: two with a redirect URL
and one without.
*/
func sections() map[string]config.Section {
	return map[string]config.Section{
		"chat":    {"driver": "stub", "redirect_url": callbackURL},
		"tracker": {"driver": "stub", "redirect_url": callbackURL},
		"bare":    {"driver": "stub"},
	}
}

/*
newManager returns a manager whose providers are faked, keeping its
attempts in a store of the test's own.
*/
func newManager(t *testing.T) (*oauthclient.Manager, *oauthclient.Fake) {
	t.Helper()

	m := oauthclient.NewManager(oauthclient.Config{Providers: sections(), AllowFakes: true}, newStore(t))
	m.Extend("stub", func(string, config.Section) (oauthclient.Driver, error) { return nil, nil })

	fake := m.Fake()

	return m, fake
}

/*
start redirects to the chat provider for the session and approves with
the grant, returning the callback query.
*/
func start(t *testing.T, m *oauthclient.Manager, fake *oauthclient.Fake, grant oauthclient.Grant) url.Values {
	t.Helper()

	authURL, state, err := m.Redirect(t.Context(), "chat", oauthclient.Request{
		Scopes: []string{"chat:write"}, UserScopes: []string{"reactions:write"}, Binding: "session-1",
		Payload: oauthclient.Payload{"return_to": "/settings"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, state)

	callback, err := fake.Approve(authURL, grant)
	require.NoError(t, err)

	parsed, err := url.Parse(callback)
	require.NoError(t, err)

	return parsed.Query()
}

func TestConnect(t *testing.T) {
	t.Parallel()

	grant := oauthclient.Grant{Access: &oauthclient.Token{Value: "secret"}, Account: "T1", App: "A1"}

	t.Run("returns the grant and the payload to the session that started", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		query := start(t, m, fake, grant)

		got, payload, err := m.Callback(t.Context(), "chat", "session-1", query)

		require.NoError(t, err)
		assert.Equal(t, grant, got)
		assert.Equal(t, oauthclient.Payload{"return_to": "/settings"}, payload)
	})

	t.Run("asks for the scopes of the request", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		authURL, _, err := m.Redirect(t.Context(), "chat", oauthclient.Request{
			Scopes: []string{"a", "b"}, UserScopes: []string{"c"}, Binding: "s", RedirectURL: "https://x.test/cb",
			Extra: map[string]string{"team": "T1"},
		})
		require.NoError(t, err)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "a b", parsed.Query().Get("scope"))
		assert.Equal(t, "c", parsed.Query().Get("user_scope"))
		assert.Equal(t, "https://x.test/cb", parsed.Query().Get("redirect_uri"))
		assert.Equal(t, "T1", parsed.Query().Get("team"))
	})

	t.Run("hands out an application token when the grant names none", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		query := start(t, m, fake, oauthclient.Grant{Account: "T1"})

		got, _, err := m.Callback(t.Context(), "chat", "session-1", query)

		require.NoError(t, err)
		require.NotNil(t, got.Access)
		assert.NotEmpty(t, got.Access.Value)
	})

	t.Run("refuses another session", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		query := start(t, m, fake, grant)

		_, _, err := m.Callback(t.Context(), "chat", "session-2", query)
		require.ErrorIs(t, err, oauthclient.ErrInvalidState)

		_, _, err = m.Callback(t.Context(), "chat", "session-1", query)
		require.NoError(t, err)
	})

	t.Run("refuses a state used twice", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		query := start(t, m, fake, grant)

		_, _, err := m.Callback(t.Context(), "chat", "session-1", query)
		require.NoError(t, err)

		_, _, err = m.Callback(t.Context(), "chat", "session-1", query)
		require.ErrorIs(t, err, oauthclient.ErrInvalidState)
	})

	t.Run("refuses a state started for another provider", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		query := start(t, m, fake, grant)

		_, _, err := m.Callback(t.Context(), "tracker", "session-1", query)
		require.ErrorIs(t, err, oauthclient.ErrInvalidState)
	})

	t.Run("refuses an unknown or missing state", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		_, _, err := m.Callback(t.Context(), "chat", "session-1", url.Values{"state": {"nope"}, "code": {"c"}})
		require.ErrorIs(t, err, oauthclient.ErrInvalidState)

		_, _, err = m.Callback(t.Context(), "chat", "", url.Values{"state": {"nope"}})
		require.ErrorIs(t, err, oauthclient.ErrInvalidState)
	})

	t.Run("refuses a code bound to another redirect url", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		authURL, _, err := m.Redirect(t.Context(), "chat", oauthclient.Request{Binding: "s"})
		require.NoError(t, err)
		other, _, err := m.Redirect(t.Context(), "chat", oauthclient.Request{Binding: "s", RedirectURL: "https://o.test"})
		require.NoError(t, err)

		callback, err := fake.Approve(other, grant)
		require.NoError(t, err)
		approved, _ := url.Parse(callback)
		first, _ := url.Parse(authURL)
		query := url.Values{"state": {first.Query().Get("state")}, "code": {approved.Query().Get("code")}}

		_, _, err = m.Callback(t.Context(), "chat", "s", query)
		require.ErrorIs(t, err, oauthclient.ErrExchange)
	})

	t.Run("reports a denial with the payload", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		authURL, _, err := m.Redirect(t.Context(), "chat", oauthclient.Request{
			Binding: "s", Payload: oauthclient.Payload{"return_to": "/back"},
		})
		require.NoError(t, err)

		callback, err := fake.Deny(authURL)
		require.NoError(t, err)
		parsed, _ := url.Parse(callback)

		_, payload, err := m.Callback(t.Context(), "chat", "s", parsed.Query())
		require.ErrorIs(t, err, oauthclient.ErrDenied)
		assert.Equal(t, "/back", payload["return_to"])
	})

	t.Run("reports a provider error and a missing code", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		for _, extra := range []url.Values{{"error": {"server_error"}}, {}} {
			authURL, _, err := m.Redirect(t.Context(), "chat", oauthclient.Request{Binding: "s"})
			require.NoError(t, err)
			parsed, _ := url.Parse(authURL)
			query := url.Values{"state": {parsed.Query().Get("state")}}
			maps.Copy(query, extra)

			_, _, err = m.Callback(t.Context(), "chat", "s", query)
			if len(extra) > 0 {
				require.ErrorIs(t, err, oauthclient.ErrProvider)
			} else {
				require.ErrorIs(t, err, oauthclient.ErrExchange)
			}
		}
	})
}

func TestRedirect(t *testing.T) {
	t.Parallel()

	t.Run("refuses an attempt without a session", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		_, _, err := m.Redirect(t.Context(), "chat", oauthclient.Request{})
		require.ErrorIs(t, err, oauthclient.ErrInvalidState)
	})

	t.Run("refuses a provider without a redirect url", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)

		_, _, err := m.Redirect(t.Context(), "bare", oauthclient.Request{Binding: "s"})
		require.ErrorIs(t, err, oauthclient.ErrMisconfigured)
	})

	t.Run("refuses an unknown provider everywhere", func(t *testing.T) {
		t.Parallel()
		m, _ := newManager(t)
		ctx := t.Context()

		_, _, err := m.Redirect(ctx, "nope", oauthclient.Request{Binding: "s"})
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
		_, _, err = m.Callback(ctx, "nope", "s", url.Values{})
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
		_, err = m.Refresh(ctx, "nope", "r")
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
		require.ErrorIs(t, m.Revoke(ctx, "nope", "t"), manager.ErrUnknownInstance)
		assert.True(t, m.Has("chat"))
		assert.False(t, m.Has("nope"))
	})

	t.Run("reports a driver that cannot build its url or a store that is down", func(t *testing.T) {
		t.Parallel()
		m := oauthclient.NewManager(oauthclient.Config{Providers: sections()}, newStore(t))
		m.Extend("stub", func(string, config.Section) (oauthclient.Driver, error) { return failing{}, nil })

		_, _, err := m.Redirect(t.Context(), "chat", oauthclient.Request{Binding: "s"})
		require.ErrorIs(t, err, oauthclient.ErrMisconfigured)
	})
}

func TestRefreshAndRevoke(t *testing.T) {
	t.Parallel()

	t.Run("refreshes a registered token once", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)
		fresh := oauthclient.Grant{Access: &oauthclient.Token{Value: "new"}}
		fake.AcceptRefresh("r1", fresh)

		got, err := m.Refresh(t.Context(), "chat", "r1")
		require.NoError(t, err)
		assert.Equal(t, fresh, got)

		_, err = m.Refresh(t.Context(), "chat", "r1")
		require.ErrorIs(t, err, oauthclient.ErrInvalidGrant)
		assert.Equal(t, []string{"r1", "r1"}, fake.Refreshed())
	})

	t.Run("records revoked tokens", func(t *testing.T) {
		t.Parallel()
		m, fake := newManager(t)

		require.NoError(t, m.Revoke(t.Context(), "chat", "t1"))
		assert.Equal(t, []string{"t1"}, fake.Revoked())
	})

	t.Run("reports a driver refusing to revoke", func(t *testing.T) {
		t.Parallel()
		m := oauthclient.NewManager(oauthclient.Config{Providers: sections()}, newStore(t))
		m.Extend("stub", func(string, config.Section) (oauthclient.Driver, error) { return failing{}, nil })

		require.ErrorIs(t, m.Revoke(t.Context(), "chat", "t"), oauthclient.ErrUnsupported)
		_, err := m.Refresh(t.Context(), "chat", "r")
		require.ErrorIs(t, err, oauthclient.ErrInvalidGrant)
	})
}

func TestFake(t *testing.T) {
	t.Parallel()

	t.Run("is refused outside tests", func(t *testing.T) {
		t.Parallel()
		m := oauthclient.NewManager(oauthclient.Config{Providers: sections()}, newStore(t))

		assert.PanicsWithValue(t, oauthclient.ErrFakesNotAllowed, func() { m.Fake() })
	})

	t.Run("refuses urls it did not make", func(t *testing.T) {
		t.Parallel()
		_, fake := newManager(t)

		for _, bad := range []string{"https://elsewhere.test/x", "https://oauthclient.test/chat/authorize", "%zz"} {
			_, err := fake.Approve(bad, oauthclient.Grant{})
			require.ErrorIs(t, err, oauthclient.ErrProvider)
			_, err = fake.Deny(bad)
			require.ErrorIs(t, err, oauthclient.ErrProvider)
		}
	})
}

func TestToken(t *testing.T) {
	t.Parallel()

	t.Run("expires within the skew", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		token := &oauthclient.Token{Value: "v", Expiry: now.Add(4 * time.Minute)}

		assert.True(t, token.Expired(now, 5*time.Minute))
		assert.False(t, token.Expired(now, time.Minute))
		assert.False(t, (&oauthclient.Token{Value: "v"}).Expired(now, time.Hour))
		assert.False(t, (*oauthclient.Token)(nil).Expired(now, time.Hour))
	})
}

/*
failing is a driver whose every call fails.
*/
type failing struct{}

func (failing) AuthURL(context.Context, oauthclient.Attempt) (string, error) {
	return "", oauthclient.ErrMisconfigured
}

func (failing) Exchange(context.Context, string, oauthclient.Attempt) (oauthclient.Grant, error) {
	return oauthclient.Grant{}, oauthclient.ErrExchange
}

func (failing) Refresh(context.Context, string) (oauthclient.Grant, error) {
	return oauthclient.Grant{}, oauthclient.ErrInvalidGrant
}

func (failing) Revoke(context.Context, string) error {
	return oauthclient.ErrUnsupported
}
