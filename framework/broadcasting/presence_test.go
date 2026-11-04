package broadcasting_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestPresenceMany(t *testing.T) {
	t.Parallel()

	t.Run("lists the listeners of each channel, none where nobody listens", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		alice := connect(t, n, "alice")
		require.Nil(t, alice.subscribe("room:1", "", 0).Error)
		bob := connect(t, n, "bob")
		require.Nil(t, bob.subscribe("room:2", "", 0).Error)

		var lists map[string][]broadcasting.Member

		require.Eventually(t, func() bool {
			var err error
			lists, err = n.server.PresenceMany(t.Context(), []string{"room:1", "room:2", "room:3"})

			return err == nil && len(lists["room:1"]) == 1 && len(lists["room:2"]) == 1
		}, waitFor, 20*time.Millisecond)
		assert.Equal(t, "alice", lists["room:1"][0].UserID)
		assert.Equal(t, "bob", lists["room:2"][0].UserID)
		assert.Empty(t, lists["room:3"])
		assert.Len(t, lists, 3)
	})

	t.Run("answers an empty list of channels with an empty map", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))

		lists, err := n.server.PresenceMany(t.Context(), nil)

		require.NoError(t, err)
		assert.Empty(t, lists)
	})

	t.Run("stops when the context is done", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := n.server.PresenceMany(ctx, []string{"room:1"})

		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestListenersChanged(t *testing.T) {
	t.Parallel()

	t.Run("tells the pattern who starts and stops listening", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		changes := make(chan string, 4)
		watch := broadcasting.ChannelOptions{
			Presence: true,
			ListenersChanged: func(_ context.Context, principal broadcasting.Principal, channel string) {
				changes <- principal.UserID + " " + channel
			},
		}
		require.NoError(t, n.server.Channels().Authorize("watch:{id}", watch, authorizeRoom))

		alice := connect(t, n, "alice")
		require.Nil(t, alice.subscribe("watch:1", "", 0).Error)
		assert.Equal(t, "alice watch:1", <-changes)

		require.Nil(t, alice.command(map[string]any{"unsubscribe": map[string]any{"channel": "watch:1"}}).Error)
		assert.Equal(t, "alice watch:1", <-changes)
	})

	t.Run("stays quiet for a refused subscription and patterns that did not ask", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		changes := make(chan string, 4)
		watch := broadcasting.ChannelOptions{
			ListenersChanged: func(_ context.Context, principal broadcasting.Principal, channel string) {
				changes <- principal.UserID + " " + channel
			},
		}
		require.NoError(t, n.server.Channels().Authorize("watch:{id}", watch, authorizeRoom))

		alice := connect(t, n, "alice")
		require.NotNil(t, alice.subscribe("watch:secret", "", 0).Error)
		require.Nil(t, alice.subscribe("room:1", "", 0).Error)
		require.Nil(t, alice.command(map[string]any{"unsubscribe": map[string]any{"channel": "room:1"}}).Error)

		assert.Empty(t, changes)
	})
}
