package oauth_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestCodeExchange(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("issues a bearer token pair that is never cached", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)

		rec := h.post(t, "/oauth/token", exchangeForm(client.ID, h.code(t, query(client.ID, nil), oauth.Approval{})))

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
		assert.Regexp(t, `"access_token":"app_oat_[0-9A-Za-z]{43}"`, rec.Body.String())
		assert.Regexp(t, `"refresh_token":"app_ort_[0-9A-Za-z]{43}"`, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"token_type":"Bearer","expires_in":3600`)
		assert.Contains(t, rec.Body.String(), `"scope":"records:read records:write"`)
	})

	t.Run("refuses a replayed code and revokes every token it issued", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		code := h.code(t, query(client.ID, nil), oauth.Approval{})
		first := h.exchange(t, client.ID, code)
		require.Equal(t, http.StatusOK, h.call(t, http.MethodGet, first.AccessToken).Code)

		status, replay := h.token(t, exchangeForm(client.ID, code))

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", replay.Error)
		rec := h.call(t, http.MethodGet, first.AccessToken)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "token_revoked", errorCode(t, rec))
		status, refreshed := h.token(t, refreshForm(client.ID, first.RefreshToken))
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", refreshed.Error)
	})

	t.Run("refuses a wrong verifier without spending the code", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		code := h.code(t, query(client.ID, nil), oauth.Approval{})
		wrong := exchangeForm(client.ID, code)
		wrong.Set("code_verifier", strings.Repeat("a", 43))

		status, out := h.token(t, wrong)

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", out.Error)
		h.exchange(t, client.ID, code)
	})

	t.Run("refuses a redirect uri other than the one the code was sent to", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		form := exchangeForm(client.ID, h.code(t, query(client.ID, nil), oauth.Approval{}))
		form.Set("redirect_uri", "https://client.example.com/other")

		status, out := h.token(t, form)

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", out.Error)
	})

	t.Run("refuses an expired code", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		code := h.code(t, query(client.ID, nil), oauth.Approval{})

		h.clock.Travel(time.Minute)
		status, out := h.token(t, exchangeForm(client.ID, code))

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", out.Error)
	})

	t.Run("refuses a code issued to another client and leaves it to its owner", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		owner, other := h.publicClient(t), h.publicClient(t)
		code := h.code(t, query(owner.ID, nil), oauth.Approval{})

		status, out := h.token(t, exchangeForm(other.ID, code))

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", out.Error)
		h.exchange(t, owner.ID, code)
	})

	t.Run("refuses codes that were never issued", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)

		for _, code := range []string{"", "app_oac_short", "app_oac_" + strings.Repeat("A", 43)} {
			status, out := h.token(t, exchangeForm(client.ID, code))

			assert.Equal(t, http.StatusBadRequest, status, code)
			assert.Equal(t, "invalid_grant", out.Error, code)
		}
	})

	t.Run("binds the grant to the approved tenant and scopes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		code := h.code(t, query(client.ID, nil), oauth.Approval{Tenant: "tenant-1", Scopes: []string{"records:read"}})

		out := h.exchange(t, client.ID, code)

		assert.Equal(t, "records:read", out.Scope)
		who := h.whoIs(t, out.AccessToken)
		assert.Equal(t, "tenant-1", who.Tenant)
		assert.Equal(t, []string{"records:read"}, who.Abilities)
		assert.Equal(t, http.StatusForbidden, h.call(t, http.MethodPost, out.AccessToken).Code)
	})
}

func TestTokenEndpoint(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	h := newHarness(t, db)
	public := h.publicClient(t)
	codeOnly := h.publicClient(t, func(c *oauth.Client) { c.GrantTypes = []string{oauth.GrantAuthorizationCode} })
	confidential, secret, err := h.server.CreateClient(t.Context(), oauth.Client{
		Name: "Server", RedirectURIs: []string{callback}, Confidential: true,
	})
	require.NoError(t, err)

	t.Run("refuses an unknown grant type", func(t *testing.T) {
		t.Parallel()
		status, out := h.token(t, url.Values{"grant_type": {"password"}, "client_id": {public.ID}})

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "unsupported_grant_type", out.Error)
	})

	t.Run("holds a client to the grants it registered", func(t *testing.T) {
		t.Parallel()
		out := h.exchange(t, codeOnly.ID, h.code(t, query(codeOnly.ID, nil), oauth.Approval{}))
		assert.Empty(t, out.RefreshToken)

		status, refused := h.token(t, refreshForm(codeOnly.ID, "app_ort_"+strings.Repeat("A", 43)))

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "unauthorized_client", refused.Error)
	})

	t.Run("refuses an unknown client", func(t *testing.T) {
		t.Parallel()
		rec := h.post(t, "/oauth/token", exchangeForm("nobody", "code"))

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
	})

	t.Run("lets a confidential client authenticate with basic or the form", func(t *testing.T) {
		t.Parallel()
		code := h.code(t, query(confidential.ID, nil), oauth.Approval{})
		status, out := h.token(t, exchangeForm("", code), url.QueryEscape(confidential.ID), url.QueryEscape(secret))
		require.Equal(t, http.StatusOK, status, out)

		form := refreshForm(confidential.ID, out.RefreshToken)
		form.Set("client_secret", secret)
		status, out = h.token(t, form)
		assert.Equal(t, http.StatusOK, status, out)
	})

	t.Run("refuses a confidential client with the wrong secret", func(t *testing.T) {
		t.Parallel()
		code := h.code(t, query(confidential.ID, nil), oauth.Approval{})

		rec := h.post(t, "/oauth/token", exchangeForm("", code), confidential.ID, "%zz")

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, `Basic realm="oauth"`, rec.Header().Get("WWW-Authenticate"))
		assert.Contains(t, rec.Body.String(), `"error":"invalid_client"`)
	})

	t.Run("refuses a body that is not a small form", func(t *testing.T) {
		t.Parallel()
		status, out := h.token(t, url.Values{"padding": {strings.Repeat("a", 17<<10)}})

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_request", out.Error)
	})
}
