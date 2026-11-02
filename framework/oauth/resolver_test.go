package oauth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestResolver(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("holds a third-party token to its tenant and scopes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		user := ids.New()
		loggedIn := h.clock.Now().Add(-time.Hour)
		out := h.exchange(t, client.ID, h.code(t, query(client.ID, map[string]string{"scope": "records:read"}),
			oauth.Approval{User: user, Tenant: "tenant-1", AuthTime: loggedIn}))

		who := h.whoIs(t, out.AccessToken)

		assert.Equal(t, user.String(), who.ID)
		assert.Equal(t, oauth.GuardThirdParty, who.Guard)
		assert.Equal(t, "tenant-1", who.Tenant)
		assert.Equal(t, []string{"records:read"}, who.Abilities)
		assert.Equal(t, loggedIn.Unix(), who.Confirmed)
		assert.Equal(t, "missing_ability", errorCode(t, h.call(t, http.MethodPost, out.AccessToken)))
	})

	t.Run("lets a first-party token act as the user everywhere for fifteen minutes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		loggedIn := h.clock.Now().Add(-time.Hour)
		form := exchangeForm(mobileID, h.code(t,
			query(mobileID, map[string]string{"redirect_uri": mobileURI, "scope": "records:read"}),
			oauth.Approval{AuthTime: loggedIn}))
		form.Set("redirect_uri", mobileURI)
		status, out := h.token(t, form)
		require.Equal(t, http.StatusOK, status, out)

		who := h.whoIs(t, out.AccessToken)

		assert.Equal(t, int64(900), out.ExpiresIn)
		assert.Equal(t, oauth.GuardFirstParty, who.Guard)
		assert.Empty(t, who.Tenant)
		assert.Equal(t, []string{"*"}, who.Abilities)
		assert.Equal(t, loggedIn.Unix(), who.Confirmed)
		assert.Equal(t, http.StatusOK, h.call(t, http.MethodPost, out.AccessToken).Code)

		h.clock.Travel(15 * time.Minute)
		assert.Equal(t, "token_expired", errorCode(t, h.call(t, http.MethodGet, out.AccessToken)))
	})

	t.Run("counts a confirmation in the app as proof of identity", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		out := h.exchange(t, client.ID, h.code(t, query(client.ID, nil),
			oauth.Approval{AuthTime: h.clock.Now().Add(-time.Hour)}))
		grant, err := ids.Parse(h.whoIs(t, out.AccessToken).Credential)
		require.NoError(t, err)

		h.clock.Travel(time.Minute)
		require.NoError(t, h.server.ConfirmGrant(t.Context(), grant))

		assert.Equal(t, h.clock.Now().Unix(), h.whoIs(t, out.AccessToken).Confirmed)
	})

	t.Run("refuses an access token past its hour", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		out := h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{}))

		h.clock.Travel(time.Hour)

		assert.Equal(t, "token_expired", errorCode(t, h.call(t, http.MethodGet, out.AccessToken)))
	})

	t.Run("refuses every token of a revoked grant", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		out := h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{}))
		grant, err := ids.Parse(h.whoIs(t, out.AccessToken).Credential)
		require.NoError(t, err)

		require.NoError(t, h.server.RevokeGrant(t.Context(), grant))
		require.NoError(t, h.server.RevokeGrant(t.Context(), grant))

		rec := h.call(t, http.MethodGet, out.AccessToken)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "token_revoked", errorCode(t, rec))
		require.ErrorIs(t, h.server.ConfirmGrant(t.Context(), grant), oauth.ErrGrantNotFound)
	})

	t.Run("turns away tokens it never issued", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)

		for _, bad := range []string{"app_oat_short", "app_oat_" + strings.Repeat("A", 43), "other_token"} {
			rec := h.call(t, http.MethodGet, bad)

			assert.Equal(t, http.StatusUnauthorized, rec.Code, bad)
			assert.Equal(t, "invalid_token", errorCode(t, rec), bad)
		}
	})
}
