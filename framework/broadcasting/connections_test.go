package broadcasting_test

import (
	"context"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestUnsubscribe(t *testing.T) {
	t.Parallel()

	t.Run("takes the user off the channel on every node, leaving the rest", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		first, second := newNode(t, client), newNode(t, client)
		c := connect(t, first, "u1")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)
		require.Nil(t, c.subscribe("room:2", "", 0).Error)

		require.NoError(t, second.server.Unsubscribe(t.Context(), "u1", "room:1"))

		assert.Equal(t, "room:1", c.unsubscribed())

		require.NoError(t, first.server.Broadcast(t.Context(), "room:1", "gone", nil))
		require.NoError(t, first.server.Broadcast(t.Context(), "room:2", "kept", nil))

		assert.Equal(t, "room:2", c.publication().Channel)
	})

	t.Run("stops when the context is done", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, n.server.Unsubscribe(ctx, "u1", "room:1"), context.Canceled)
	})
}

func TestShutdown(t *testing.T) {
	t.Parallel()
	n := newNode(t, testkit.NewValkey(t))
	c := connect(t, n, "u1")
	code := make(chan websocket.StatusCode, 1)

	go func() { code <- c.closed() }()

	require.NoError(t, n.server.Shutdown(t.Context()))
	assert.Equal(t, websocket.StatusCode(3001), <-code, "clients are told to reconnect elsewhere")
}
