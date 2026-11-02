package oauth_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestValidateAuthorization(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	h := newHarness(t, db)
	client := h.publicClient(t)
	twoURIs := h.publicClient(t, func(c *oauth.Client) {
		c.RedirectURIs = []string{callback, "https://client.example.com/other"}
	})
	cli := h.publicClient(t, func(c *oauth.Client) { c.RedirectURIs = []string{"http://127.0.0.1/cb"} })

	t.Run("accepts a valid request and defaults to every scope of the client", func(t *testing.T) {
		t.Parallel()
		req, err := h.server.ValidateAuthorization(t.Context(), query(client.ID, nil))

		require.NoError(t, err)
		assert.Equal(t, client.ID, req.Client.ID)
		assert.Equal(t, callback, req.RedirectURI)
		assert.Equal(t, "xyz", req.State)
		assert.Equal(t, challenge, req.CodeChallenge)
		assert.Equal(t, []string{"records:read", "records:write"}, req.Scopes)
	})

	t.Run("keeps the scopes asked for", func(t *testing.T) {
		t.Parallel()
		req, err := h.server.ValidateAuthorization(t.Context(),
			query(client.ID, map[string]string{"scope": "records:read records:write records:read"}))

		require.NoError(t, err)
		assert.Equal(t, []string{"records:read", "records:write"}, req.Scopes)
	})

	t.Run("uses the only registered uri when none is given", func(t *testing.T) {
		t.Parallel()
		req, err := h.server.ValidateAuthorization(t.Context(), query(client.ID, map[string]string{"redirect_uri": ""}))

		require.NoError(t, err)
		assert.Equal(t, callback, req.RedirectURI)
	})

	t.Run("matches a loopback uri on any port", func(t *testing.T) {
		t.Parallel()
		req, err := h.server.ValidateAuthorization(t.Context(),
			query(cli.ID, map[string]string{"redirect_uri": "http://127.0.0.1:53682/cb"}))

		require.NoError(t, err)
		assert.Equal(t, "http://127.0.0.1:53682/cb", req.RedirectURI)
	})

	pages := map[string]url.Values{
		"an unknown client":              query("nobody", nil),
		"a missing client":               query("", map[string]string{"client_id": ""}),
		"a repeated client id":           withExtra(query(client.ID, nil), "client_id", client.ID),
		"an unregistered redirect uri":   query(client.ID, map[string]string{"redirect_uri": "https://evil.example/cb"}),
		"a registered uri with a query":  query(client.ID, map[string]string{"redirect_uri": callback + "?next=1"}),
		"no uri when two are registered": query(twoURIs.ID, map[string]string{"redirect_uri": ""}),
		"a loopback uri on another path": query(cli.ID, map[string]string{"redirect_uri": "http://127.0.0.1:1/other"}),
		"a loopback uri on another host": query(cli.ID, map[string]string{"redirect_uri": "http://localhost:1/cb"}),
		"a broken loopback uri":          query(cli.ID, map[string]string{"redirect_uri": "http://127.0.0.1:x/cb"}),
	}

	for name, q := range pages {
		t.Run("shows an error page instead of redirecting for "+name, func(t *testing.T) {
			t.Parallel()
			_, err := h.server.ValidateAuthorization(t.Context(), q)

			var refusal *oauth.Error
			require.ErrorAs(t, err, &refusal)
			_, redirects := refusal.RedirectURL()
			assert.False(t, redirects)
		})
	}

	ask := func(name, value string) url.Values { return query(client.ID, map[string]string{name: value}) }
	redirects := map[string]struct {
		query url.Values
		code  string
	}{
		"an implicit grant":     {ask("response_type", "token"), "unsupported_response_type"},
		"a missing challenge":   {ask("code_challenge", ""), "invalid_request"},
		"the plain method":      {ask("code_challenge_method", "plain"), "invalid_request"},
		"a missing method":      {ask("code_challenge_method", ""), "invalid_request"},
		"a short challenge":     {ask("code_challenge", "abc"), "invalid_request"},
		"a repeated scope":      {withExtra(ask("scope", "records:read"), "scope", "records:read"), "invalid_request"},
		"an unregistered scope": {ask("scope", "records:read admin"), "invalid_scope"},
	}

	for name, tc := range redirects {
		t.Run("redirects the refusal of "+name+" with state and issuer", func(t *testing.T) {
			t.Parallel()
			_, err := h.server.ValidateAuthorization(t.Context(), tc.query)

			var refusal *oauth.Error
			require.ErrorAs(t, err, &refusal)
			to, ok := refusal.RedirectURL()
			require.True(t, ok)
			u, err := url.Parse(to)
			require.NoError(t, err)
			assert.Equal(t, "client.example.com", u.Host)
			assert.Equal(t, tc.code, u.Query().Get("error"))
			assert.Equal(t, "xyz", u.Query().Get("state"))
			assert.Equal(t, issuer, u.Query().Get("iss"))
			assert.Contains(t, refusal.Error(), tc.code)
		})
	}
}

func withExtra(q url.Values, name, value string) url.Values {
	q.Add(name, value)

	return q
}

func TestApprove(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	h := newHarness(t, db)
	client := h.publicClient(t, func(c *oauth.Client) { c.RedirectURIs = []string{callback + "?app=1"} })

	request := func(t *testing.T, clientID, redirect string) *oauth.AuthorizationRequest {
		t.Helper()

		req, err := h.server.ValidateAuthorization(t.Context(),
			query(clientID, map[string]string{"redirect_uri": redirect, "scope": "records:read"}))
		require.NoError(t, err)

		return req
	}

	t.Run("redirects with the code, the state and the issuer, keeping the uri's query", func(t *testing.T) {
		t.Parallel()
		to, err := h.server.Approve(t.Context(), request(t, client.ID, callback+"?app=1"),
			oauth.Approval{User: ids.New(), Tenant: "tenant-1"})

		require.NoError(t, err)
		u, err := url.Parse(to)
		require.NoError(t, err)
		assert.Equal(t, "1", u.Query().Get("app"))
		assert.Equal(t, "xyz", u.Query().Get("state"))
		assert.Equal(t, issuer, u.Query().Get("iss"))
		assert.Regexp(t, `^app_oac_[0-9A-Za-z]{43}$`, u.Query().Get("code"))
	})

	t.Run("tells the client when the user refuses", func(t *testing.T) {
		t.Parallel()
		u, err := url.Parse(h.server.Deny(request(t, client.ID, "")))

		require.NoError(t, err)
		assert.Equal(t, "access_denied", u.Query().Get("error"))
		assert.Equal(t, "xyz", u.Query().Get("state"))
		assert.Equal(t, issuer, u.Query().Get("iss"))
	})

	refused := map[string]struct {
		clientID string
		approval oauth.Approval
	}{
		"no user": {client.ID, oauth.Approval{}},
		"scopes that were not asked for": {
			client.ID, oauth.Approval{User: ids.New(), Scopes: []string{"records:write"}},
		},
		"a tenant on a first-party grant": {mobileID, oauth.Approval{User: ids.New(), Tenant: "tenant-1"}},
	}

	for name, tc := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()
			redirect := ""
			if tc.clientID == mobileID {
				redirect = mobileURI
			}

			_, err := h.server.Approve(t.Context(), request(t, tc.clientID, redirect), tc.approval)

			require.ErrorIs(t, err, oauth.ErrInvalidApproval)
		})
	}
}
