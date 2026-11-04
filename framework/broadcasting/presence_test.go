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

func TestPresence(t *testing.T) {
	t.Parallel()

	t.Run("lists the listeners of a channel across nodes", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		first, second := newNode(t, client), newNode(t, client)
		alice := connect(t, first, "alice")
		require.Nil(t, alice.subscribe("room:1", "", 0).Error)
		bob := connect(t, second, "bob")
		require.Nil(t, bob.subscribe("room:1", "", 0).Error)

		for alice.join() != "bob" {
			continue
		}

		var members []broadcasting.Member

		require.Eventually(t, func() bool {
			var err error
			members, err = first.server.Presence(t.Context(), "room:1")

			return err == nil && len(members) == 2
		}, waitFor, 20*time.Millisecond)
		assert.Equal(t, "alice", members[0].UserID)
		assert.Equal(t, "bob", members[1].UserID)
		assert.JSONEq(t, `{"name":"bob"}`, string(members[1].Info))
		assert.NotEmpty(t, members[0].ClientID)
	})

	t.Run("answers a client only on channels it is subscribed to", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := connect(t, n, "alice")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)

		reply := c.command(map[string]any{"presence": map[string]any{"channel": "room:1"}})
		require.Nil(t, reply.Error)
		require.NotNil(t, reply.Presence)
		require.Len(t, reply.Presence.Presence, 1)

		for _, info := range reply.Presence.Presence {
			assert.Equal(t, "alice", info.User)
		}

		denied := c.command(map[string]any{"presence": map[string]any{"channel": "room:2"}})
		require.NotNil(t, denied.Error)
		assert.Equal(t, 103, denied.Error.Code)
	})

	t.Run("stops when the context is done", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := n.server.Presence(ctx, "room:1")

		require.ErrorIs(t, err, context.Canceled)
	})
}

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
