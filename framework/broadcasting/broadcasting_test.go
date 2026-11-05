package broadcasting_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("requires a resolver", func(t *testing.T) {
		t.Parallel()

		_, err := broadcasting.Open(t.Context(), testkit.NewValkey(t), broadcasting.Config{})

		require.ErrorIs(t, err, broadcasting.ErrNoResolver)
	})

	t.Run("serves both endpoints", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))

		handlers := n.server.Handlers()

		assert.Contains(t, handlers, broadcasting.PathWebsocket)
		assert.Contains(t, handlers, broadcasting.PathUniSSE)
	})

	t.Run("logs through the context logger", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))

		_, err := upgrade(t, n, http.Header{"Origin": []string{"https://elsewhere.example"}})

		require.Error(t, err, "an upgrade from another origin is refused")
		assert.Contains(t, n.logs.String(), `"msg":"realtime"`)
	})
}

func TestChannels(t *testing.T) {
	t.Parallel()

	t.Run("rejects malformed patterns", func(t *testing.T) {
		t.Parallel()
		channels := &broadcasting.Channels{}

		for _, pattern := range []string{"room}", "room:{", "room:{}", "a:{x}:{x}", "a:{x{y}}"} {
			err := channels.Authorize(pattern, broadcasting.ChannelOptions{}, authorizeRoom)
			require.ErrorIs(t, err, broadcasting.ErrInvalidPattern, pattern)
			assert.Panics(t, func() { channels.MustAuthorize(pattern, broadcasting.ChannelOptions{}, authorizeRoom) })
		}
	})

	t.Run("hands placeholder values to the authorizer", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))

		got := make(chan map[string]string, 1)
		require.NoError(t, n.server.Channels().Authorize("team:{team}:board:{board}", broadcasting.ChannelOptions{},
			func(_ context.Context, _ broadcasting.Principal, params map[string]string) (bool, error) {
				got <- params

				return true, nil
			}))

		c := connect(t, n, "u1")
		reply := c.subscribe("team:7:board:main", "", 0)

		require.Nil(t, reply.Error)
		assert.Equal(t, map[string]string{"team": "7", "board": "main"}, <-got)

		require.NoError(t, n.server.Broadcast(t.Context(), "team:7:board:main", "moved", nil))
		assert.Equal(t, "moved", decode(t, c.publication().Pub).Event, "a channel without history still delivers")
	})
}

func TestBroadcast(t *testing.T) {
	t.Parallel()

	t.Run("reaches a subscriber on another node", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		first, second := newNode(t, client), newNode(t, client)
		c := connect(t, first, "u1")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)

		payload := map[string]string{"text": "hi"}
		require.NoError(t, second.server.Broadcast(t.Context(), "room:1", "note.added", payload))

		p := c.publication()
		e := decode(t, p.Pub)
		assert.Equal(t, "room:1", p.Channel)
		assert.Equal(t, "note.added", e.Event)
		assert.JSONEq(t, `{"text":"hi"}`, string(e.Data))
		assert.False(t, e.ServerTime.IsZero())
	})

	t.Run("publishes one event on several channels", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := connect(t, n, "u1")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)
		require.Nil(t, c.subscribe("room:2", "", 0).Error)

		require.NoError(t, n.server.BroadcastMany(t.Context(), []string{"room:1", "room:2"}, "ping", nil))

		channels := []string{c.publication().Channel, c.publication().Channel}
		assert.ElementsMatch(t, []string{"room:1", "room:2"}, channels)
	})

	t.Run("carries the message id and meta", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := connect(t, n, "u1")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)

		m := broadcasting.Message{ID: "evt_1", Event: "note.added", Meta: map[string]string{"by": "u2"}}
		require.NoError(t, n.server.Publish(t.Context(), []string{"room:1"}, m))

		e := decode(t, c.publication().Pub)
		assert.Equal(t, "evt_1", e.ID)
		assert.Equal(t, map[string]string{"by": "u2"}, e.Meta)
	})

	t.Run("leaves out an empty id and meta", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := connect(t, n, "u1")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)

		require.NoError(t, n.server.Broadcast(t.Context(), "room:1", "plain", nil))

		raw := string(c.publication().Pub.Data)
		assert.NotContains(t, raw, `"id"`)
		assert.NotContains(t, raw, `"meta"`)
	})

	t.Run("keeps history under the client's key prefix", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		n := newNode(t, client)

		require.NoError(t, n.server.Broadcast(t.Context(), "room:1", "kept", nil))

		rdb := client.Rueidis()
		pattern := strings.TrimSuffix(client.Prefix(broadcasting.DefaultKeyspace), ":") + ".*"
		keys, err := rdb.Do(t.Context(), rdb.B().Keys().Pattern(pattern).Build()).AsStrSlice()
		require.NoError(t, err)
		assert.NotEmpty(t, keys)
	})

	t.Run("recovers missed broadcasts after a reconnect", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		first := connect(t, n, "u1")
		sub := first.subscribe("room:1", "", 0).Subscribe
		require.True(t, sub.Recoverable)
		require.NoError(t, n.server.Broadcast(t.Context(), "room:1", "one", nil))
		seen := first.publication().Pub.Offset
		require.NoError(t, first.conn.Close(websocket.StatusNormalClosure, ""))

		require.NoError(t, n.server.Broadcast(t.Context(), "room:1", "two", nil))
		require.NoError(t, n.server.Broadcast(t.Context(), "room:1", "three", nil))

		again := connect(t, n, "u1")
		recovered := again.subscribe("room:1", sub.Epoch, seen).Subscribe
		require.True(t, recovered.Recovered)
		require.Len(t, recovered.Publications, 2)
		assert.Equal(t, "two", decode(t, &recovered.Publications[0]).Event)
		assert.Equal(t, "three", decode(t, &recovered.Publications[1]).Event)
	})

	t.Run("keeps ephemeral broadcasts out of history", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		live := connect(t, n, "u1")
		sub := live.subscribe("room:1", "", 0).Subscribe

		require.NoError(t, n.server.BroadcastEphemeral(t.Context(), "room:1", "typing", nil))
		require.NoError(t, n.server.Broadcast(t.Context(), "room:1", "kept", nil))

		assert.Equal(t, "typing", decode(t, live.publication().Pub).Event)
		assert.Equal(t, "kept", decode(t, live.publication().Pub).Event)

		late := connect(t, n, "u2")
		recovered := late.subscribe("room:1", sub.Epoch, sub.Offset).Subscribe
		require.Len(t, recovered.Publications, 1)
		assert.Equal(t, "kept", decode(t, &recovered.Publications[0]).Event)
	})

	t.Run("refuses an unknown channel", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))

		require.ErrorIs(t, n.server.Broadcast(t.Context(), "lobby", "x", nil), broadcasting.ErrUnknownChannel)
		require.ErrorIs(t, n.server.BroadcastEphemeral(t.Context(), "lobby", "x", nil), broadcasting.ErrUnknownChannel)
	})

	t.Run("refuses a payload over the cap", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		payload := strings.Repeat("x", broadcasting.DefaultMaxPayload)

		require.ErrorIs(t, n.server.Broadcast(t.Context(), "room:1", "big", payload), broadcasting.ErrPayloadTooLarge)
		require.ErrorIs(t, n.server.BroadcastEphemeral(t.Context(), "room:1", "big", payload),
			broadcasting.ErrPayloadTooLarge)
	})

	t.Run("refuses a payload that cannot be encoded", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))

		require.ErrorContains(t, n.server.Broadcast(t.Context(), "room:1", "bad", make(chan int)), "encode bad")
	})

	t.Run("stops when the context is done", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, n.server.Broadcast(ctx, "room:1", "x", nil), context.Canceled)
		require.ErrorIs(t, n.server.BroadcastEphemeral(ctx, "room:1", "x", nil), context.Canceled)
	})
}

func TestSubscribe(t *testing.T) {
	t.Parallel()

	t.Run("refuses what the authorizer denies", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := connect(t, n, "u1")

		assert.Equal(t, 103, c.subscribe("room:secret", "", 0).Error.Code)
		assert.Equal(t, 103, c.subscribe("user:u2", "", 0).Error.Code)
		assert.Nil(t, c.subscribe("user:u1", "", 0).Error)
	})

	t.Run("refuses an unknown channel", func(t *testing.T) {
		t.Parallel()
		c := connect(t, newNode(t, testkit.NewValkey(t)), "u1")

		assert.Equal(t, 102, c.subscribe("lobby", "", 0).Error.Code)
	})

	t.Run("refuses when the authorizer fails", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c := connect(t, n, "u1")

		assert.Equal(t, 100, c.subscribe("room:broken", "", 0).Error.Code)
		assert.Contains(t, n.logs.String(), "authorize realtime channel")
	})

	t.Run("refuses publishing from a client", func(t *testing.T) {
		t.Parallel()
		c := connect(t, newNode(t, testkit.NewValkey(t)), "u1")
		require.Nil(t, c.subscribe("room:1", "", 0).Error)

		publish := map[string]any{"channel": "room:1", "data": json.RawMessage(`{}`)}
		reply := c.command(map[string]any{"publish": publish})

		require.NotNil(t, reply.Error)
	})

	t.Run("accepts an anonymous connection the resolver allows", func(t *testing.T) {
		t.Parallel()
		c := connect(t, newNode(t, testkit.NewValkey(t)), "anonymous")

		assert.Nil(t, c.subscribe("room:1", "", 0).Error)
	})

	t.Run("subscribes the principal's channels on connect", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		c, reply := dial(t, n, "u1|user:u1")
		require.Contains(t, reply.Connect.Subs, "user:u1")

		require.NoError(t, n.server.Broadcast(t.Context(), "user:u1", "hello", nil))

		assert.Equal(t, "hello", decode(t, c.publication().Pub).Event)
	})
}

func TestConnect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		token string
		code  int
		log   string
	}{
		{name: "refuses a connection without credentials", token: "", code: 101},
		{name: "refuses when the resolver fails", token: "fail", code: 100, log: "resolve realtime connection"},
		{name: "refuses info that cannot be encoded", token: "badinfo", code: 100, log: "encode realtime"},
		{name: "refuses a denied channel of the principal", token: "u1|room:secret", code: 103},
		{name: "refuses an unknown channel of the principal", token: "u1|lobby", code: 102},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := newNode(t, testkit.NewValkey(t))

			_, reply := dial(t, n, tc.token)

			require.NotNil(t, reply.Error)
			assert.Equal(t, tc.code, reply.Error.Code)
			assert.Contains(t, n.logs.String(), tc.log)
		})
	}
}

func TestDisconnect(t *testing.T) {
	t.Parallel()

	t.Run("closes every connection of the user on every node", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		first, second := newNode(t, client), newNode(t, client)
		c := connect(t, first, "u1")
		other := connect(t, first, "u2")

		require.NoError(t, second.server.Disconnect(t.Context(), "u1"))

		assert.Equal(t, websocket.StatusCode(4009), c.closed())

		assert.Nil(t, other.subscribe("room:1", "", 0).Error, "other users stay connected")
	})

	t.Run("stops when the context is done", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, testkit.NewValkey(t))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, n.server.Disconnect(ctx, "u1"), context.Canceled)
	})
}
