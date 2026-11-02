package github_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/github"
	"github.com/dimmihq/dimmi/framework/socialite/drivertest"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func newClient(t *testing.T) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	return client
}

func newUser() socialite.User {
	return socialite.User{
		Provider:      "github",
		ID:            "583231",
		Email:         "ada@example.com",
		EmailVerified: true,
		Name:          "Ada Lovelace",
		AvatarURL:     "https://avatars.githubusercontent.com/u/583231",
	}
}

/*
newServer starts a provider serving the GitHub user and emails APIs
with the bodies given, to callers holding a token it issued.
*/
func newServer(t *testing.T, user, emails any) *drivertest.Server {
	t.Helper()

	srv := drivertest.NewServer(t, newUser())
	serve := func(body any) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !srv.Authorized(r) || r.Header.Get("X-GitHub-Api-Version") == "" {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(body))
		}
	}

	srv.Handle("GET /user", serve(user))
	srv.Handle("GET /user/emails", serve(emails))

	return srv
}

func newGitHubServer(t *testing.T) *drivertest.Server {
	t.Helper()

	u := newUser()

	return newServer(t,
		map[string]any{"id": 583231, "login": "ada", "name": u.Name, "avatar_url": u.AvatarURL},
		[]map[string]any{
			{"email": "old@example.com", "primary": false, "verified": true},
			{"email": u.Email, "primary": true, "verified": true},
		},
	)
}

func newProvider(t *testing.T, srv *drivertest.Server) *github.Provider {
	t.Helper()

	provider, err := github.New("github", newClient(t), github.Options{
		ClientID:     srv.ClientID,
		ClientSecret: srv.ClientSecret,
		AuthURL:      srv.URL + "/authorize",
		TokenURL:     srv.URL + "/token",
		APIURL:       srv.URL,
	})
	require.NoError(t, err)

	return provider
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	srv := newGitHubServer(t)

	return drivertest.Harness{Provider: newProvider(t, srv), User: srv.User, Approve: srv.Approve}
}

func TestProvider(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

/*
signIn signs in at the server and returns the user the provider reads.
*/
func signIn(t *testing.T, srv *drivertest.Server) (socialite.User, error) {
	t.Helper()

	p := newProvider(t, srv)
	attempt, code := drivertest.SignIn(t, drivertest.Harness{Provider: p, Approve: srv.Approve})
	token, err := p.Exchange(t.Context(), code, attempt)
	require.NoError(t, err)

	return p.User(t.Context(), token)
}

func TestUser(t *testing.T) {
	t.Parallel()

	t.Run("reports a primary address github has not verified", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t,
			map[string]any{"id": 1, "login": "ada", "email": "public@example.com"},
			[]map[string]any{{"email": "ada@example.com", "primary": true, "verified": false}},
		)

		user, err := signIn(t, srv)

		require.NoError(t, err)
		assert.Equal(t, "ada@example.com", user.Email)
		assert.False(t, user.EmailVerified)
		assert.Equal(t, "ada", user.Name)
		assert.Equal(t, "public@example.com", user.Raw["email"])
	})

	t.Run("refuses a profile without an id", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, map[string]any{"login": "ada"}, []any{})

		_, err := signIn(t, srv)

		require.ErrorIs(t, err, socialite.ErrProvider)
	})

	t.Run("reports an api answering garbage", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, map[string]any{"id": 1}, map[string]any{"not": "a list"})

		_, err := signIn(t, srv)

		require.ErrorIs(t, err, socialite.ErrProvider)
	})

	t.Run("reports a refused token", func(t *testing.T) {
		t.Parallel()
		p := newProvider(t, newGitHubServer(t))

		_, err := p.User(t.Context(), socialite.Token{AccessToken: "revoked"})

		require.ErrorIs(t, err, socialite.ErrProvider)
		require.ErrorContains(t, err, "answered 401")
	})

	t.Run("reports an unreachable api", func(t *testing.T) {
		t.Parallel()
		p, err := github.New("github", newClient(t), github.Options{
			ClientID: "c", ClientSecret: "s", APIURL: "http://127.0.0.1:1",
		})
		require.NoError(t, err)

		_, err = p.User(t.Context(), socialite.Token{AccessToken: "t"})

		require.ErrorIs(t, err, socialite.ErrProvider)
	})
}

func TestManager(t *testing.T) {
	t.Parallel()

	t.Run("signs a user in from redirect to callback", func(t *testing.T) {
		t.Parallel()
		srv := newGitHubServer(t)
		store, err := cache.Open(testkit.NewValkey(t), cache.Options{Keyspace: "cache"})
		require.NoError(t, err)
		t.Cleanup(store.Close)

		m := socialite.NewManager(socialite.Config{Providers: map[string]config.Section{
			"github": {
				"driver":        "github",
				"client_id":     srv.ClientID,
				"client_secret": srv.ClientSecret,
				"auth_url":      srv.URL + "/authorize",
				"token_url":     srv.URL + "/token",
				"api_url":       srv.URL,
				"redirect_url":  drivertest.RedirectURL,
			},
		}}, store)
		m.Extend("github", github.Factory(newClient(t)))

		authURL, state, err := m.Redirect(t.Context(), "github", "")
		require.NoError(t, err)
		assert.Contains(t, authURL, "scope=read%3Auser+user%3Aemail")
		callback, err := url.Parse(srv.Approve(t, authURL))
		require.NoError(t, err)

		user, err := m.Callback(t.Context(), "github", state, callback.Query())

		require.NoError(t, err)
		assert.Equal(t, "583231", user.ID)
		assert.Equal(t, "ada@example.com", user.Email)
		assert.True(t, user.EmailVerified)
		assert.NotEmpty(t, user.Token.AccessToken)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("refuses broken configuration", func(t *testing.T) {
		t.Parallel()
		factory := github.Factory(newClient(t))

		_, err := factory("github", config.Section{"client_id": "c"})
		require.ErrorIs(t, err, socialite.ErrMisconfigured)

		_, err = factory("github", config.Section{"client_id": map[string]any{"not": "a string"}})
		require.Error(t, err)
	})
}
