package auth_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivertest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/tests"
)

/*
gitlab is a provider the product does not ship, written the way a new
driver would be, to show a driver alone adds a way to sign in. It signs
in whoever the code names.
*/
type gitlab struct{}

func (gitlab) Key() string                { return "gitlab" }
func (gitlab) Protocol() string           { return providers.ProtocolOAuth2 }
func (gitlab) Display() providers.Display { return providers.Display{Name: "GitLab"} }

func (gitlab) AuthorizeURL(context.Context) (providers.Authorization, error) {
	return providers.Authorization{URL: "https://gitlab.test/oauth/authorize?state=gitlab-state", State: "gitlab-state"}, nil
}

func (gitlab) Callback(_ context.Context, state string, query url.Values) (providers.ExternalIdentity, error) {
	if query.Get("state") != state {
		return providers.ExternalIdentity{}, providers.ErrState
	}

	return providers.ExternalIdentity{
		Provider: "gitlab", Subject: query.Get("code"), Email: query.Get("code") + "@example.com", EmailVerified: true,
		Username: query.Get("code"),
	}, nil
}

/*
writeJSON answers a provider API request with the body.
*/
func writeJSON(t *testing.T, w fhttp.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

/*
providerAt points the provider's client and endpoints at the local
provider, as an operator configures a self-hosted one.
*/
func providerAt(key string, srv *drivertest.Server, endpoints ...string) tests.Option {
	env := []string{
		"DIMMI_HTTP_CLIENT__ALLOW_PRIVATE_NETWORKS=true",
		"DIMMI_AUTH__OAUTH__" + upper(key) + "__CLIENT_ID=" + srv.ClientID,
		"DIMMI_AUTH__OAUTH__" + upper(key) + "__CLIENT_SECRET=" + srv.ClientSecret,
	}

	for _, endpoint := range endpoints {
		env = append(env, "DIMMI_SOCIALITE__PROVIDERS__"+upper(key)+"__"+endpoint)
	}

	return tests.WithEnv(env...)
}

func upper(key string) string {
	return map[string]string{"github": "GITHUB", "google": "GOOGLE"}[key]
}

/*
signInAt runs a sign-in against the local provider and returns the
callback's response.
*/
func signInAt(t *testing.T, tc *tests.TestCase, srv *drivertest.Server, provider string) *testkit.Response {
	t.Helper()

	attempt := tc.StartOAuth(tc.Client(), "/auth/"+provider+"/redirect")
	callback := srv.Approve(t, attempt.AuthURL)

	return tc.Client().WithCookie(attempt.Cookie).Send(fhttp.MethodGet, callback, nil)
}

func TestIdentityProviderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("signs in with a newly registered driver and its configuration alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t,
			tests.WithEnv("DIMMI_AUTH__OAUTH__GITLAB__CLIENT_ID=gitlab-client"),
			tests.Before(func(a *app.App) { require.NoError(t, a.IdentityProviders.Register(gitlab{})) }),
		)

		tc.Client().GetJSON("/api/v1/auth/config").AssertJSONPath("data.providers", []testkit.JSON{{
			"key": "gitlab", "name": "GitLab", "protocol": "oauth2", "redirect_url": "/auth/gitlab/redirect",
		}})
		attempt := tc.StartOAuth(tc.Client(), "/auth/gitlab/redirect")
		assert.Equal(t, "https://gitlab.test/oauth/authorize?state=gitlab-state", attempt.AuthURL)

		res := tc.Client().WithCookie(attempt.Cookie).
			Send(fhttp.MethodGet, "/auth/gitlab/callback?code=grace&state=gitlab-state", nil)

		assertLands(t, res, "/")
		tc.Browser(res).GetJSON("/api/v1/me").AssertJSONPath("data.auth.auth_methods", []any{"gitlab"})
		tc.Table("users").AssertHas(testkit.Row{"email": "grace@example.com", "created_via": "gitlab"})
		tc.Table("oauth_identities").AssertHas(testkit.Row{"provider": "gitlab", "username": "grace"})
	})

	t.Run("refuses to register two drivers under one key", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t,
			tests.Before(func(a *app.App) { require.NoError(t, a.IdentityProviders.Register(gitlab{})) }),
			tests.Before(func(a *app.App) {
				require.Error(t, a.IdentityProviders.Register(gitlab{}))
				require.NoError(t, a.IdentityProviders.Register(namedGitHub{}))
			}),
		)

		require.ErrorContains(t, err, `"github"`)
	})
}

/*
namedGitHub is a driver that claims the key of a shipped one.
*/
type namedGitHub struct{ gitlab }

func (namedGitHub) Key() string { return "github" }

func TestGitHubProvider(t *testing.T) {
	t.Parallel()

	newServer := func(t *testing.T, emails []testkit.JSON) *drivertest.Server {
		t.Helper()

		srv := drivertest.NewServer(t, socialite.User{})
		srv.Handle("GET /user", func(w fhttp.ResponseWriter, r *fhttp.Request) {
			require.True(t, srv.Authorized(r))
			writeJSON(t, w, testkit.JSON{"id": 583231, "login": "ghopper", "name": "Grace Hopper"})
		})
		srv.Handle("GET /user/emails", func(w fhttp.ResponseWriter, r *fhttp.Request) {
			require.True(t, srv.Authorized(r))
			writeJSON(t, w, emails)
		})

		return srv
	}

	endpoints := func(srv *drivertest.Server) tests.Option {
		return providerAt("github", srv,
			"AUTH_URL="+srv.URL+"/authorize", "TOKEN_URL="+srv.URL+"/token", "API_URL="+srv.URL)
	}

	t.Run("signs up with the primary verified address and keeps the login for display", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, []testkit.JSON{
			{"email": "old@example.com", "primary": false, "verified": true},
			{"email": "grace@example.com", "primary": true, "verified": true},
		})
		tc := tests.New(t, endpoints(srv))

		assertLands(t, signInAt(t, tc, srv, "github"), "/")
		tc.Table("oauth_identities").AssertHas(testkit.Row{
			"provider": "github", "provider_user_id": "583231", "username": "ghopper", "email": "grace@example.com",
		})
	})

	t.Run("refuses an account whose primary address GitHub has not verified", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, []testkit.JSON{{"email": "grace@example.com", "primary": true, "verified": false}})
		tc := tests.New(t, endpoints(srv))

		assertLands(t, signInAt(t, tc, srv, "github"), "/login?error=oauth_email_unverified")
		tc.Table("oauth_identities").AssertCount(0)
	})
}

func TestGoogleProvider(t *testing.T) {
	t.Parallel()

	newServer := func(t *testing.T) (*drivertest.Server, tests.Option) {
		t.Helper()

		srv := drivertest.NewServer(t, socialite.User{
			ID: "110169484474386276334", Email: "grace@acme.example", EmailVerified: true, Name: "Grace Hopper",
		})

		return srv, providerAt("google", srv, "ISSUER="+srv.URL)
	}

	t.Run("signs up with the verified ID token and keeps the account's domain", func(t *testing.T) {
		t.Parallel()
		srv, option := newServer(t)
		srv.TamperIDToken(func(claims *jwt.Claims) { claims.Extra["hd"] = "acme.example" })
		tc := tests.New(t, option)

		res := signInAt(t, tc, srv, "google")

		assertLands(t, res, "/")
		tc.Table("oauth_identities").AssertHas(testkit.Row{
			"provider": "google", "provider_user_id": "110169484474386276334", "hosted_domain": "acme.example",
		})
	})

	t.Run("refuses an ID token issued for another attempt", func(t *testing.T) {
		t.Parallel()
		srv, option := newServer(t)
		srv.TamperIDToken(func(claims *jwt.Claims) { claims.Extra["nonce"] = "another-attempt" })
		tc := tests.New(t, option)

		assertLands(t, signInAt(t, tc, srv, "google"), "/login?error=oauth_unavailable")
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@acme.example"})
	})

	t.Run("refuses an address Google has not verified", func(t *testing.T) {
		t.Parallel()
		srv, option := newServer(t)
		srv.TamperIDToken(func(claims *jwt.Claims) { claims.Extra["email_verified"] = false })
		tc := tests.New(t, option)

		assertLands(t, signInAt(t, tc, srv, "google"), "/login?error=oauth_email_unverified")
	})
}
