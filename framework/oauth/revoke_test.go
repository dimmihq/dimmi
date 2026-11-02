package oauth_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestRevocation(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	start := func(t *testing.T) (*harness, oauth.Client, tokens) {
		t.Helper()

		h := newHarness(t, db)
		client := h.publicClient(t)

		out := h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{}))

		return h, client, out
	}

	revoke := func(t *testing.T, h *harness, clientID, token string) {
		t.Helper()

		rec := h.post(t, "/oauth/revoke", url.Values{"client_id": {clientID}, "token": {token}})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	}

	t.Run("ends an access token alone", func(t *testing.T) {
		t.Parallel()
		h, client, out := start(t)

		revoke(t, h, client.ID, out.AccessToken)

		assert.Equal(t, "token_revoked", errorCode(t, h.call(t, http.MethodGet, out.AccessToken)))
		status, refreshed := h.token(t, refreshForm(client.ID, out.RefreshToken))
		assert.Equal(t, http.StatusOK, status, refreshed)
	})

	t.Run("ends the whole grant for a refresh token", func(t *testing.T) {
		t.Parallel()
		h, client, out := start(t)

		revoke(t, h, client.ID, out.RefreshToken)

		assert.Equal(t, "token_revoked", errorCode(t, h.call(t, http.MethodGet, out.AccessToken)))
		status, refreshed := h.token(t, refreshForm(client.ID, out.RefreshToken))
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", refreshed.Error)
	})

	t.Run("answers 200 for tokens it does not know", func(t *testing.T) {
		t.Parallel()
		h, client, _ := start(t)

		for _, token := range []string{"garbage", "app_oat_" + strings.Repeat("A", 43)} {
			revoke(t, h, client.ID, token)
		}
	})

	t.Run("answers 200 for another client's token and leaves it alone", func(t *testing.T) {
		t.Parallel()
		h, _, out := start(t)
		other := h.publicClient(t)

		revoke(t, h, other.ID, out.AccessToken)

		assert.Equal(t, http.StatusOK, h.call(t, http.MethodGet, out.AccessToken).Code)
	})

	t.Run("needs a token and an authenticated client", func(t *testing.T) {
		t.Parallel()
		h, client, out := start(t)

		missing := h.post(t, "/oauth/revoke", url.Values{"client_id": {client.ID}})
		unknown := h.post(t, "/oauth/revoke", url.Values{"client_id": {"nobody"}, "token": {out.AccessToken}})
		huge := h.post(t, "/oauth/revoke", url.Values{"token": {strings.Repeat("a", 17<<10)}})

		assert.Equal(t, http.StatusBadRequest, missing.Code)
		assert.Contains(t, missing.Body.String(), `"error":"invalid_request"`)
		assert.Equal(t, http.StatusUnauthorized, unknown.Code)
		assert.Equal(t, http.StatusBadRequest, huge.Code)
		assert.Equal(t, http.StatusOK, h.call(t, http.MethodGet, out.AccessToken).Code)
	})
}
