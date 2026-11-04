package accountauth_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/accountauth"
)

const grantEndpoint = "https://idp.test/token"

func newClient(t *testing.T) (*accountauth.Client, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	fake := httpclienttest.New(client).PreventStrayRequests()

	return accountauth.New(accountauth.Config{
		ClientID: "id", ClientSecret: "secret", AuthURL: "https://idp.test/auth", TokenURL: grantEndpoint,
		RedirectURL: "https://app.test/cb", Scopes: []string{"read"}, Params: map[string]string{"access_type": "offline"},
	}, client), fake
}

func tokenAnswer(t *testing.T, fields map[string]any) httpclient.Response {
	t.Helper()

	body, err := json.Marshal(fields)
	require.NoError(t, err)

	return httpclient.Response{
		Status: http.StatusOK, Body: body, Header: http.Header{"Content-Type": {"application/json"}},
	}
}

func TestAuthorizationURL(t *testing.T) {
	t.Parallel()

	t.Run("carries the state, the proof key, the params and extra scopes", func(t *testing.T) {
		t.Parallel()
		client, _ := newClient(t)
		verifier := accountauth.NewVerifier()

		parsed, err := url.Parse(client.AuthorizationURL("state-1", verifier, "shared"))
		require.NoError(t, err)

		query := parsed.Query()
		assert.Equal(t, "state-1", query.Get("state"))
		assert.Equal(t, "S256", query.Get("code_challenge_method"))
		assert.NotEmpty(t, query.Get("code_challenge"))
		assert.NotContains(t, parsed.String(), verifier)
		assert.Equal(t, "offline", query.Get("access_type"))
		assert.Equal(t, "read shared", query.Get("scope"))
	})
}

func TestTokens(t *testing.T) {
	t.Parallel()

	t.Run("exchanges a code with its proof", func(t *testing.T) {
		t.Parallel()
		client, fake := newClient(t)
		fake.Stub("POST "+grantEndpoint, tokenAnswer(t, map[string]any{
			"access_token": "a1", "refresh_token": "r1", "expires_in": 3600, "scope": "read send",
		}))

		tokens, err := client.Exchange(t.Context(), "code", "verifier")

		require.NoError(t, err)
		assert.Equal(t, "a1", tokens.AccessToken)
		assert.Equal(t, "r1", tokens.RefreshToken)
		assert.Equal(t, []string{"read", "send"}, tokens.Scopes)
		assert.False(t, tokens.Expiry.IsZero())
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			values, err := url.ParseQuery(string(req.Body))
			require.NoError(t, err)

			return values.Get("code_verifier") == "verifier"
		})
	})

	t.Run("hands back a rotated refresh token and keeps the old one otherwise", func(t *testing.T) {
		t.Parallel()
		client, fake := newClient(t)
		fake.Stub("POST "+grantEndpoint, tokenAnswer(t, map[string]any{"access_token": "a2", "refresh_token": "r2"}))

		tokens, err := client.Refresh(t.Context(), "r1")
		require.NoError(t, err)
		assert.Equal(t, "r2", tokens.RefreshToken)

		stable, stableFake := newClient(t)
		stableFake.Stub("POST "+grantEndpoint, tokenAnswer(t, map[string]any{"access_token": "a2"}))

		tokens, err = stable.Refresh(t.Context(), "r1")
		require.NoError(t, err)
		assert.Equal(t, "r1", tokens.RefreshToken)
	})

	t.Run("tells why a grant failed", func(t *testing.T) {
		t.Parallel()

		cases := map[string]error{
			`{"error":"invalid_grant"}`: mailbox.ErrGrantRevoked,
			`{"error":"invalid_grant","error_description":"AADSTS70008: expired"}`:   mailbox.ErrGrantRevoked,
			`{"error":"admin_policy_enforced"}`:                                      mailbox.ErrAdminBlocked,
			`{"error":"invalid_client","error_description":"AADSTS65004: declined"}`: mailbox.ErrAdminBlocked,
		}

		for body, want := range cases {
			client, fake := newClient(t)
			fake.Stub("POST "+grantEndpoint, httpclient.Response{
				Status: http.StatusBadRequest, Body: []byte(body),
				Header: http.Header{"Content-Type": {"application/json"}},
			})

			_, err := client.Refresh(t.Context(), "r1")
			require.ErrorIs(t, err, want, body)
		}
	})

	t.Run("reports other failures as passing", func(t *testing.T) {
		t.Parallel()
		client, fake := newClient(t)
		fake.Stub("POST "+grantEndpoint, httpclient.Response{Status: http.StatusBadGateway, Body: []byte("oops")})

		_, err := client.Exchange(t.Context(), "code", "verifier")
		require.Error(t, err)
		assert.NotErrorIs(t, err, mailbox.ErrGrantRevoked)

		stray, _ := newClient(t)
		_, err = stray.Refresh(t.Context(), "r1")
		require.Error(t, err)
	})
}
