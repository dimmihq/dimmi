package oauth_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivertest"
	"github.com/dimmihq/dimmi/framework/socialite/internal/oauth"
)

/*
newClient returns a client registered with the server, sending its
exchange to the token URL.
*/
func newClient(t *testing.T, srv *drivertest.Server, tokenURL string, scopes ...string) oauth.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	return oauth.Client{
		HTTP: client,
		Config: oauth2.Config{
			ClientID:     srv.ClientID,
			ClientSecret: srv.ClientSecret,
			Scopes:       scopes,
			Endpoint:     oauth2.Endpoint{AuthURL: srv.URL + "/authorize", TokenURL: tokenURL},
		},
	}
}

/*
approve signs in for a fresh attempt and returns it with its code.
*/
func approve(t *testing.T, srv *drivertest.Server, client oauth.Client) (socialite.Attempt, string) {
	t.Helper()

	attempt := drivertest.NewAttempt()
	callback, err := url.Parse(srv.Approve(t, client.AuthURL(attempt)))
	require.NoError(t, err)

	return attempt, callback.Query().Get("code")
}

func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("trades a code for tokens with the verifier", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, socialite.User{ID: "1"})
		client := newClient(t, srv, srv.URL+"/token")
		attempt, code := approve(t, srv, client)

		token, err := client.Exchange(t.Context(), code, attempt)

		require.NoError(t, err)
		assert.NotEmpty(t, token.AccessToken)
		assert.NotEmpty(t, token.RefreshToken)
		assert.True(t, token.Expiry.After(clock.System().Now()))
		assert.Empty(t, token.IDToken)
	})

	t.Run("returns the id token of an openid sign-in", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, socialite.User{ID: "1"})
		client := newClient(t, srv, srv.URL+"/token", "openid")
		attempt, code := approve(t, srv, client)

		token, err := client.Exchange(t.Context(), code, attempt)

		require.NoError(t, err)
		assert.NotEmpty(t, token.IDToken)
	})

	t.Run("reports a refused code", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, socialite.User{ID: "1"})
		client := newClient(t, srv, srv.URL+"/token")

		_, err := client.Exchange(t.Context(), "made-up", drivertest.NewAttempt())

		require.ErrorIs(t, err, socialite.ErrExchange)
		require.ErrorContains(t, err, "400 invalid_grant")
	})

	t.Run("reports a failing provider", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, socialite.User{ID: "1"})
		srv.Handle("POST /broken", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})
		client := newClient(t, srv, srv.URL+"/broken")

		_, err := client.Exchange(t.Context(), "code", drivertest.NewAttempt())

		require.ErrorIs(t, err, socialite.ErrProvider)
	})

	t.Run("reports an unreachable provider", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, socialite.User{ID: "1"})
		client := newClient(t, srv, "http://127.0.0.1:1/token")

		_, err := client.Exchange(t.Context(), "code", drivertest.NewAttempt())

		require.ErrorIs(t, err, socialite.ErrProvider)
	})
}
