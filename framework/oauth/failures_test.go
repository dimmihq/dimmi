package oauth_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestStoreFailures(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	h := newHarness(t, db)
	client := h.publicClient(t)
	req, err := h.server.ValidateAuthorization(t.Context(), query(client.ID, nil))
	require.NoError(t, err)
	out := h.exchange(t, client.ID, h.code(t, query(client.ID, nil), oauth.Approval{}))
	db.Close()

	t.Run("answer server_error rather than a verdict", func(t *testing.T) {
		t.Parallel()
		code := "app_oac_" + strings.Repeat("A", 43)
		responses := []int{
			h.post(t, "/oauth/token", exchangeForm(client.ID, code)).Code,
			h.post(t, "/oauth/token", exchangeForm(mobileID, code)).Code,
			h.post(t, "/oauth/token", refreshForm(mobileID, out.RefreshToken)).Code,
			h.post(t, "/oauth/revoke", url.Values{"client_id": {mobileID}, "token": {out.AccessToken}}).Code,
			h.call(t, http.MethodGet, out.AccessToken).Code,
		}

		for _, status := range responses {
			assert.Equal(t, http.StatusInternalServerError, status)
		}

		rec, body := h.register(t, `{"client_name":"Agent","redirect_uris":["`+callback+`"]}`)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "server_error", body.Error)
	})

	t.Run("surface every store error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		_, err := h.server.ValidateAuthorization(ctx, query(client.ID, nil))
		require.Error(t, err)
		_, err = h.server.Approve(ctx, req, oauth.Approval{User: ids.New()})
		require.Error(t, err)
		require.Error(t, h.server.RevokeGrant(ctx, ids.New()))
		require.Error(t, h.server.ConfirmGrant(ctx, ids.New()))
	})
}
