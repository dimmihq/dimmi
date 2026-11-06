package broadcasting_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
cappedResolver authenticates any token as that user, allowed two
connections.
*/
func cappedResolver(_ context.Context, _ *http.Request, token string) (broadcasting.Principal, error) {
	return broadcasting.Principal{UserID: token, MaxConnections: 2}, nil
}

func TestConnectionLimit(t *testing.T) {
	t.Parallel()

	t.Run("closes the oldest connection past the limit, on every node", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		cfg := broadcasting.Config{Resolve: cappedResolver}
		first, second := nodeWith(t, client, cfg), nodeWith(t, client, cfg)

		oldest := connect(t, first, "u1")
		middle := connect(t, second, "u1")
		stranger := connect(t, first, "u2")
		newest := connect(t, second, "u1")

		assert.Equal(t, websocket.StatusCode(4004), oldest.closed())
		require.Nil(t, middle.subscribe("room:1", "", 0).Error)
		require.Nil(t, newest.subscribe("room:1", "", 0).Error)
		assert.Nil(t, stranger.subscribe("room:1", "", 0).Error)
	})

	t.Run("leaves connections alone under the limit", func(t *testing.T) {
		t.Parallel()
		n := nodeWith(t, testkit.NewValkey(t), broadcasting.Config{Resolve: cappedResolver})
		a, b := connect(t, n, "u1"), connect(t, n, "u1")

		assert.Nil(t, a.subscribe("room:1", "", 0).Error)
		assert.Nil(t, b.subscribe("room:1", "", 0).Error)
	})
}
