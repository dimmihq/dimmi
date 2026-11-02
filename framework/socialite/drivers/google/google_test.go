package google_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/google"
	"github.com/dimmihq/dimmi/framework/socialite/drivertest"
)

func newClient(t *testing.T) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	return client
}

/*
newHarness runs the Google provider against a local server standing in
for accounts.google.com, built through the factory as configuration
would.
*/
func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	srv := drivertest.NewServer(t, socialite.User{
		Provider:      "google",
		ID:            "110169484474386276334",
		Email:         "ada@example.com",
		EmailVerified: true,
		Name:          "Ada Lovelace",
		AvatarURL:     "https://lh3.googleusercontent.com/a/ada",
	})

	provider, err := google.Factory(newClient(t), clock.System())("google", config.Section{
		"client_id":     srv.ClientID,
		"client_secret": srv.ClientSecret,
		"issuer":        srv.URL,
	})
	require.NoError(t, err)

	return drivertest.Harness{Provider: provider, User: srv.User, Approve: srv.Approve}
}

func TestProvider(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestAuthURL(t *testing.T) {
	t.Parallel()

	t.Run("asks for the account chooser and the identity scopes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		authURL, err := h.Provider.AuthURL(t.Context(), drivertest.NewAttempt())
		require.NoError(t, err)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "select_account", parsed.Query().Get("prompt"))
		assert.Equal(t, "openid email profile", parsed.Query().Get("scope"))
		assert.NotEmpty(t, parsed.Query().Get("nonce"))
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("defaults to the google issuer", func(t *testing.T) {
		t.Parallel()

		_, err := google.Factory(newClient(t), clock.System())("google", config.Section{"client_id": "c"})
		require.NoError(t, err)
	})

	t.Run("refuses broken configuration", func(t *testing.T) {
		t.Parallel()
		factory := google.Factory(newClient(t), clock.System())

		_, err := factory("google", config.Section{})
		require.ErrorIs(t, err, socialite.ErrMisconfigured)

		_, err = factory("google", config.Section{"client_id": map[string]any{"not": "a string"}})
		require.Error(t, err)
	})
}
