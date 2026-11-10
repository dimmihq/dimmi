package oauth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestGrants(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("lists a person's live grants with their app and tenant", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		user := ids.New()
		h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{User: user, Tenant: "tenant-1"}))

		grants, err := h.server.Grants(t.Context(), user)
		require.NoError(t, err)

		require.Len(t, grants, 1)
		assert.Equal(t, client.ID, grants[0].ClientID)
		assert.Equal(t, client.Name, grants[0].ClientName)
		assert.Equal(t, "tenant-1", grants[0].Tenant)
		assert.False(t, grants[0].LastUsedAt.IsZero())
	})

	t.Run("ends a person's grant and its tokens", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		user := ids.New()
		out := h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{User: user, Tenant: "t"}))
		grants, err := h.server.Grants(t.Context(), user)
		require.NoError(t, err)

		require.NoError(t, h.server.RevokeUserGrant(t.Context(), user, grants[0].ID))

		left, err := h.server.Grants(t.Context(), user)
		require.NoError(t, err)
		assert.Empty(t, left)
		assert.Equal(t, 401, h.call(t, "GET", out.AccessToken).Code)
	})

	t.Run("refuses to end someone else's grant, or one already ended", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		client := h.publicClient(t)
		user := ids.New()
		h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{User: user, Tenant: "t"}))
		grants, err := h.server.Grants(t.Context(), user)
		require.NoError(t, err)

		require.ErrorIs(t, h.server.RevokeUserGrant(t.Context(), ids.New(), grants[0].ID), oauth.ErrGrantNotFound)
		require.NoError(t, h.server.RevokeUserGrant(t.Context(), user, grants[0].ID))
		require.ErrorIs(t, h.server.RevokeUserGrant(t.Context(), user, grants[0].ID), oauth.ErrGrantNotFound)
	})
}
