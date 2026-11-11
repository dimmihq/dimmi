package hosted_test

import (
	"maps"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivers/hosted"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivertest"
)

/*
newDriver configures a hosted provider whose endpoints name the account
in their query, against the local server.
*/
func newDriver(t *testing.T, srv *drivertest.Server, extra config.Section) oauthclient.Driver {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	section := config.Section{
		"client_id": srv.ClientID, "client_secret": srv.ClientSecret,
		"auth_url": srv.URL + "/authorize?site={account}", "token_url": srv.URL + "/token?site={account}",
	}
	maps.Copy(section, extra)

	driver, err := hosted.Factory(client)("helpdesk", section)
	require.NoError(t, err)

	return driver
}

/*
attemptFor is a fresh attempt naming the account.
*/
func attemptFor(account string) oauthclient.Attempt {
	attempt := drivertest.NewAttempt("read")
	attempt.Extra = map[string]string{hosted.AccountParam: account}

	return attempt
}

func TestHosted(t *testing.T) {
	t.Parallel()

	t.Run("the browser goes to the account's own host and the account is not sent on", func(t *testing.T) {
		t.Parallel()

		srv := drivertest.NewServer(t)
		authURL, err := newDriver(t, srv, nil).AuthURL(t.Context(), attemptFor(" Acme "))
		require.NoError(t, err)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "acme", parsed.Query().Get("site"))
		assert.Empty(t, parsed.Query().Get(hosted.AccountParam))
		assert.NotEmpty(t, parsed.Query().Get("code_challenge"))
	})

	for name, extra := range map[string]config.Section{
		"pkce in the form": nil,
		"basic auth":       {"header_auth": true, "pkce": false},
	} {
		t.Run("a code is traded for a grant naming the account with "+name, func(t *testing.T) {
			t.Parallel()

			srv := drivertest.NewServer(t)
			driver := newDriver(t, srv, extra)
			attempt := attemptFor("acme")

			authURL, err := driver.AuthURL(t.Context(), attempt)
			require.NoError(t, err)

			callback, err := url.Parse(srv.Approve(t, authURL))
			require.NoError(t, err)

			grant, err := driver.Exchange(t.Context(), callback.Query().Get("code"), attempt)
			require.NoError(t, err)
			assert.NotNil(t, grant.Access)
			assert.Equal(t, "acme", grant.Account)
		})
	}

	t.Run("an attempt without a valid account is refused", func(t *testing.T) {
		t.Parallel()

		driver := newDriver(t, drivertest.NewServer(t), nil)
		for _, account := range []string{"", "evil.example.com/x", "-acme"} {
			_, err := driver.AuthURL(t.Context(), attemptFor(account))
			require.ErrorIs(t, err, hosted.ErrAccount)

			_, err = driver.Exchange(t.Context(), "code", attemptFor(account))
			require.ErrorIs(t, err, hosted.ErrAccount)
		}
	})

	t.Run("a refused code is an error", func(t *testing.T) {
		t.Parallel()

		_, err := newDriver(t, drivertest.NewServer(t), nil).Exchange(t.Context(), "unknown", attemptFor("acme"))
		require.Error(t, err)
	})

	t.Run("a refresh reaches the account its token carries", func(t *testing.T) {
		t.Parallel()

		srv := drivertest.NewServer(t)
		driver := newDriver(t, srv, nil)
		attempt := attemptFor("acme")

		authURL, err := driver.AuthURL(t.Context(), attempt)
		require.NoError(t, err)
		callback, err := url.Parse(srv.Approve(t, authURL))
		require.NoError(t, err)

		grant, err := driver.Exchange(t.Context(), callback.Query().Get("code"), attempt)
		require.NoError(t, err)
		require.NotNil(t, grant.Refresh)
		assert.True(t, strings.HasPrefix(grant.Refresh.Value, "acme "))

		fresh, err := driver.Refresh(t.Context(), grant.Refresh.Value)
		require.NoError(t, err)
		assert.Equal(t, "acme", fresh.Account)
		assert.True(t, strings.HasPrefix(fresh.Refresh.Value, "acme "))
	})

	t.Run("a refresh token without a valid account is refused and revoke is unsupported", func(t *testing.T) {
		t.Parallel()

		driver := newDriver(t, drivertest.NewServer(t), nil)
		for _, refresh := range []string{"rt", "evil.example.com rt"} {
			_, err := driver.Refresh(t.Context(), refresh)
			require.ErrorIs(t, err, oauthclient.ErrInvalidGrant)
		}

		_, err := driver.Refresh(t.Context(), "acme unknown")
		require.Error(t, err)
		require.ErrorIs(t, driver.Revoke(t.Context(), "at"), oauthclient.ErrUnsupported)
	})

	t.Run("endpoints without the account placeholder are refused", func(t *testing.T) {
		t.Parallel()

		client, err := httpclient.New(httpclient.Config{})
		require.NoError(t, err)

		fixed := "https://helpdesk.example"
		_, err = hosted.Factory(client)("helpdesk", config.Section{
			"client_id": "id", "auth_url": fixed + "/authorize", "token_url": fixed + "/{account}",
		})
		require.ErrorIs(t, err, oauthclient.ErrMisconfigured)

		_, err = hosted.Factory(client)("helpdesk", config.Section{"client_id": []int{1}})
		require.Error(t, err)
	})
}
