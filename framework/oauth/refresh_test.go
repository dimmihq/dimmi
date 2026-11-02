package oauth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestRefresh(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	start := func(t *testing.T) (*harness, oauth.Client, tokens) {
		t.Helper()

		h := newHarness(t, db)
		client := h.publicClient(t)

		out := h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{}))

		return h, client, out
	}

	refresh := func(t *testing.T, h *harness, clientID, refreshToken string) tokens {
		t.Helper()

		status, out := h.token(t, refreshForm(clientID, refreshToken))
		require.Equal(t, http.StatusOK, status, out)

		return out
	}

	t.Run("rotates the refresh token and keeps earlier access tokens alive", func(t *testing.T) {
		t.Parallel()
		h, client, first := start(t)

		second := refresh(t, h, client.ID, first.RefreshToken)
		third := refresh(t, h, client.ID, second.RefreshToken)

		assert.NotEqual(t, first.RefreshToken, second.RefreshToken)
		assert.NotEqual(t, second.AccessToken, third.AccessToken)
		assert.Equal(t, "records:read records:write", third.Scope)
		assert.Equal(t, http.StatusOK, h.call(t, http.MethodGet, first.AccessToken).Code)
	})

	t.Run("revokes the whole family when a spent refresh token comes back", func(t *testing.T) {
		t.Parallel()
		h, client, first := start(t)
		second := refresh(t, h, client.ID, first.RefreshToken)

		status, replay := h.token(t, refreshForm(client.ID, first.RefreshToken))

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", replay.Error)
		for _, access := range []string{first.AccessToken, second.AccessToken} {
			assert.Equal(t, "token_revoked", errorCode(t, h.call(t, http.MethodGet, access)))
		}

		status, out := h.token(t, refreshForm(client.ID, second.RefreshToken))
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", out.Error)
	})

	t.Run("narrows the access token without narrowing the grant", func(t *testing.T) {
		t.Parallel()
		h, client, first := start(t)
		form := refreshForm(client.ID, first.RefreshToken)
		form.Set("scope", "records:read")

		status, narrow := h.token(t, form)

		require.Equal(t, http.StatusOK, status, narrow)
		assert.Equal(t, "records:read", narrow.Scope)
		assert.Equal(t, "missing_ability", errorCode(t, h.call(t, http.MethodPost, narrow.AccessToken)))
		wide := refresh(t, h, client.ID, narrow.RefreshToken)
		assert.Equal(t, http.StatusOK, h.call(t, http.MethodPost, wide.AccessToken).Code)
	})

	t.Run("refuses scopes the grant does not hold", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		first := h.exchange(t, client.ID, h.code(t, query(client.ID, map[string]string{"scope": "records:read"}),
			oauth.Approval{}))
		form := refreshForm(client.ID, first.RefreshToken)
		form.Set("scope", "records:read records:write")

		status, out := h.token(t, form)

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_scope", out.Error)
		refresh(t, h, client.ID, first.RefreshToken)
	})

	t.Run("refuses a refresh token left unused too long", func(t *testing.T) {
		t.Parallel()
		h, client, first := start(t)

		h.clock.Travel(90 * 24 * time.Hour)
		status, out := h.token(t, refreshForm(client.ID, first.RefreshToken))

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", out.Error)
	})

	t.Run("refuses another client's refresh token without revoking it", func(t *testing.T) {
		t.Parallel()
		h, client, first := start(t)
		other := h.publicClient(t)

		status, out := h.token(t, refreshForm(other.ID, first.RefreshToken))

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "invalid_grant", out.Error)
		refresh(t, h, client.ID, first.RefreshToken)
	})

	t.Run("refuses refresh tokens that were never issued", func(t *testing.T) {
		t.Parallel()
		h, client, first := start(t)

		for _, bad := range []string{"", first.AccessToken, "app_ort_" + strings.Repeat("A", 43)} {
			status, out := h.token(t, refreshForm(client.ID, bad))

			assert.Equal(t, http.StatusBadRequest, status, bad)
			assert.Equal(t, "invalid_grant", out.Error, bad)
		}
	})
}
