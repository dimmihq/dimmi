package generic_test

import (
	"maps"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivers/generic"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivertest"
)

func newHTTP(t *testing.T) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	return client
}

/*
section configures a generic provider against the local server.
*/
func section(srv *drivertest.Server, extra config.Section) config.Section {
	s := config.Section{
		"client_id": srv.ClientID, "client_secret": srv.ClientSecret, "auth_url": srv.URL + "/authorize",
		"token_url": srv.URL + "/token", "revoke_url": srv.URL + "/revoke",
	}
	maps.Copy(s, extra)

	return s
}

/*
endpoints points the token and revocation endpoints at the address.
*/
func endpoints(address string) config.Section {
	return config.Section{"token_url": address, "revoke_url": address}
}

func newDriver(t *testing.T, srv *drivertest.Server, extra config.Section) oauthclient.Driver {
	t.Helper()

	driver, err := generic.Factory(newHTTP(t))("tracker", section(srv, extra))
	require.NoError(t, err)

	return driver
}

func TestDriver(t *testing.T) {
	t.Parallel()

	for name, extra := range map[string]config.Section{
		"pkce in the form": {},
		"basic auth":       {"header_auth": true},
		"no pkce":          {"pkce": false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			drivertest.Run(t, func(t *testing.T) drivertest.Harness {
				t.Helper()
				srv := drivertest.NewServer(t)

				return drivertest.Harness{
					Driver: newDriver(t, srv, extra), Approve: srv.Approve, Refreshes: true, Revokes: true,
				}
			})
		})
	}
}

func TestGeneric(t *testing.T) {
	t.Parallel()

	t.Run("asks for the default scopes with the separator and extra parameters", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t)
		driver := newDriver(t, srv, config.Section{"scopes": []any{"read", "write"}, "scope_separator": ","})
		attempt := drivertest.NewAttempt()
		attempt.Extra = map[string]string{"prompt": "consent"}

		authURL, err := driver.AuthURL(t.Context(), attempt)
		require.NoError(t, err)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "read,write", parsed.Query().Get("scope"))
		assert.Equal(t, "consent", parsed.Query().Get("prompt"))
		assert.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
	})

	t.Run("returns the granted scopes and tokens", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t)
		h := drivertest.Harness{Driver: newDriver(t, srv, nil), Approve: srv.Approve}

		grant := drivertest.Connect(t, h)

		assert.Equal(t, []string{"read"}, grant.Scopes)
		require.NotNil(t, grant.Refresh)
		assert.False(t, grant.Access.Expiry.IsZero())
		assert.Equal(t, "Bearer", grant.Raw["token_type"])
	})

	t.Run("revokes at the provider", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t)
		h := drivertest.Harness{Driver: newDriver(t, srv, nil), Approve: srv.Approve}
		grant := drivertest.Connect(t, h)

		require.NoError(t, h.Driver.Revoke(t.Context(), grant.Access.Value))
		assert.Equal(t, []string{grant.Access.Value}, srv.Revoked())
	})

	t.Run("says it cannot revoke without a revocation url", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t)
		driver := newDriver(t, srv, config.Section{"revoke_url": ""})

		require.ErrorIs(t, driver.Revoke(t.Context(), "t"), oauthclient.ErrUnsupported)
	})

	t.Run("reports a failing or unreachable provider", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t)
		srv.Handle("POST /broken", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})
		broken := newDriver(t, srv, endpoints(srv.URL+"/broken"))
		gone := newDriver(t, srv, endpoints("http://127.0.0.1:1/gone"))

		_, err := broken.Exchange(t.Context(), "code", drivertest.NewAttempt())
		require.ErrorIs(t, err, oauthclient.ErrProvider)
		_, err = broken.Refresh(t.Context(), "r")
		require.ErrorIs(t, err, oauthclient.ErrProvider)
		require.ErrorIs(t, broken.Revoke(t.Context(), "t"), oauthclient.ErrProvider)
		require.ErrorIs(t, gone.Revoke(t.Context(), "t"), oauthclient.ErrProvider)
	})

	t.Run("refuses an incomplete configuration", func(t *testing.T) {
		t.Parallel()

		_, err := generic.Factory(newHTTP(t))("tracker", config.Section{"client_id": "x"})
		require.ErrorIs(t, err, oauthclient.ErrMisconfigured)

		_, err = generic.Factory(newHTTP(t))("tracker", config.Section{"scopes": map[string]any{"bad": 1}})
		require.Error(t, err)
	})
}
