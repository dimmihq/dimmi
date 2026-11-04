package testkit_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
realtimeServer starts a realtime node whose connections are named by
their token and whose "room:{id}" channels track presence and keep
history, and returns the node and its base URL.
*/
func realtimeServer(t *testing.T) (*broadcasting.Server, string) {
	t.Helper()

	resolve := func(_ context.Context, r *http.Request, token string) (broadcasting.Principal, error) {
		user := token
		if user == "" {
			user = r.Header.Get("X-User")
		}

		if user == "" {
			return broadcasting.Principal{}, broadcasting.ErrUnauthenticated
		}

		principal := broadcasting.Principal{UserID: user, Info: map[string]string{"name": user}}
		if user == "signed-out" {
			principal.Alive = func(context.Context) (bool, error) { return false, nil }
		}

		return principal, nil
	}

	logger, err := log.New(log.Options{Driver: "json", Level: "error", Writer: io.Discard})
	require.NoError(t, err)

	ctx := log.NewContext(t.Context(), logger)
	server, err := broadcasting.Open(ctx, testkit.NewValkey(t), broadcasting.Config{Resolve: resolve})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Shutdown(context.WithoutCancel(t.Context())) })

	room := broadcasting.ChannelOptions{Presence: true, JoinLeave: true, HistorySize: 10, HistoryTTL: time.Minute}
	server.Channels().MustAuthorize("room:{id}", room,
		func(_ context.Context, _ broadcasting.Principal, params map[string]string) (bool, error) {
			return params["id"] != "secret", nil
		})

	mux := http.NewServeMux()
	for path, handler := range server.Handlers() {
		mux.Handle(path, handler)
	}

	return server, testkit.Serve(t, mux)
}

func TestRealtime(t *testing.T) {
	t.Parallel()

	t.Run("receives broadcasts with their envelope", func(t *testing.T) {
		t.Parallel()
		server, url := realtimeServer(t)
		c := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "ada"})
		require.Nil(t, c.Subscribe("room:1").Error)

		m := broadcasting.Message{
			ID: "evt_1", Event: "note.added", Data: map[string]int{"n": 1}, Meta: map[string]string{"k": "v"},
		}
		require.NoError(t, server.Publish(t.Context(), []string{"room:1"}, m))

		p := c.Publication()
		assert.Equal(t, "room:1", p.Channel)
		assert.Equal(t, "evt_1", p.ID)
		assert.Equal(t, "note.added", p.Event)
		assert.JSONEq(t, `{"n":1}`, string(p.Data))
		assert.Equal(t, "v", p.Meta["k"])
	})

	t.Run("authenticates with the upgrade request's headers", func(t *testing.T) {
		t.Parallel()
		_, url := realtimeServer(t)

		c := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Header: http.Header{"X-User": {"bob"}}})

		assert.Nil(t, c.Subscribe("room:1").Error)
	})

	t.Run("reports a refused connection and subscription", func(t *testing.T) {
		t.Parallel()
		_, url := realtimeServer(t)

		_, refusal := testkit.DialRealtime(t, url, testkit.RealtimeOptions{})
		require.NotNil(t, refusal)
		assert.Equal(t, 101, refusal.Code)

		c := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "ada"})
		assert.Equal(t, 103, c.Subscribe("room:secret").Error.Code)
		assert.NotNil(t, c.Publish("room:1", map[string]int{}))
	})

	t.Run("reports the server closing a connection for what it asked", func(t *testing.T) {
		t.Parallel()
		_, url := realtimeServer(t)

		c := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "signed-out"})

		assert.Equal(t, 3005, c.ClosedBySubscribing("room:1"))
	})

	t.Run("recovers what it missed", func(t *testing.T) {
		t.Parallel()
		server, url := realtimeServer(t)
		first := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "ada"})
		sub := first.Subscribe("room:1")
		first.Close()

		require.NoError(t, server.Broadcast(t.Context(), "room:1", "missed", nil))

		again := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "ada"})
		recovered := again.Recover("room:1", sub.Epoch, sub.Offset)
		require.True(t, recovered.Recovered)
		require.Len(t, recovered.Publications, 1)
		assert.Equal(t, "missed", recovered.Publications[0].Event)
	})

	t.Run("lists who listens and sees arrivals", func(t *testing.T) {
		t.Parallel()
		_, url := realtimeServer(t)
		ada := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "ada"})
		require.Nil(t, ada.Subscribe("room:1").Error)
		bob := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "bob"})
		require.Nil(t, bob.Subscribe("room:1").Error)

		for ada.Join() != "bob" {
			continue
		}

		listeners, refusal := ada.Presence("room:1")
		require.Nil(t, refusal)
		assert.Len(t, listeners, 2)

		_, refusal = ada.Presence("room:2")
		assert.NotNil(t, refusal)
	})

	t.Run("sees the server close the connection", func(t *testing.T) {
		t.Parallel()
		server, url := realtimeServer(t)
		c := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: "ada"})

		require.NoError(t, server.Disconnect(t.Context(), "ada"))

		assert.Equal(t, 4009, c.Closed())
	})
}
