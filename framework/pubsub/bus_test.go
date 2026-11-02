package pubsub_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/pubsub"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
wait bounds how long a test waits for a notice that should arrive.
*/
const wait = 5 * time.Second

/*
received collects the payloads a subscription receives.
*/
type received chan string

func (in received) handle(payload []byte) {
	in <- string(payload)
}

func newBus(t *testing.T, client *redis.Client) *pubsub.Bus {
	t.Helper()

	return pubsub.New(client, pubsub.Options{Keyspace: "ps"})
}

func subscribe(t *testing.T, bus *pubsub.Bus, topic string) (received, pubsub.Unsubscribe) {
	t.Helper()

	in := make(received, 16)
	unsubscribe, err := bus.Subscribe(t.Context(), topic, in.handle)
	require.NoError(t, err)
	t.Cleanup(unsubscribe)

	return in, unsubscribe
}

func receive(t *testing.T, in received) string {
	t.Helper()

	select {
	case got := <-in:
		return got
	case <-clock.System().After(wait):
		require.Fail(t, "no notice arrived")

		return ""
	}
}

/*
channels picks plain or sharded channels for a bus under test.
*/
type channels struct {
	name    string
	sharded bool
}

func (c channels) open(t *testing.T, client *redis.Client) *pubsub.Bus {
	t.Helper()

	bus := newBus(t, client)
	if c.sharded {
		pubsub.UseShardedChannels(bus)
	}

	return bus
}

/*
subscribers returns how many connections listen to the topic on the
server.
*/
func (c channels) subscribers(t *testing.T, client *redis.Client, topic string) int64 {
	t.Helper()

	rdb := client.Rueidis()
	channel := client.Key("ps", topic)
	cmd := rdb.B().PubsubNumsub().Channel(channel).Build()
	if c.sharded {
		cmd = rdb.B().PubsubShardnumsub().Channel(channel).Build()
	}

	counts, err := rdb.Do(t.Context(), cmd).AsIntMap()
	require.NoError(t, err)

	return counts[channel]
}

func TestBus(t *testing.T) {
	t.Parallel()

	for _, c := range []channels{{name: "plain channels"}, {name: "sharded channels", sharded: true}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testDelivery(t, c)
		})
	}
}

func testDelivery(t *testing.T, c channels) {
	t.Helper()

	t.Run("delivers payloads byte for byte", func(t *testing.T) {
		t.Parallel()
		bus := c.open(t, testkit.NewValkey(t))
		in, _ := subscribe(t, bus, "settings")

		require.NoError(t, bus.Publish(t.Context(), "settings", []byte{0, 1, 0xff, '{'}))

		assert.Equal(t, string([]byte{0, 1, 0xff, '{'}), receive(t, in))
	})

	t.Run("reaches other processes on the same prefix", func(t *testing.T) {
		t.Parallel()
		cfg := testkit.ValkeyConfig(t)
		listener, publisher := openClient(t, cfg), openClient(t, cfg)
		in, _ := subscribe(t, c.open(t, listener), "cache.bust")

		require.NoError(t, c.open(t, publisher).Publish(t.Context(), "cache.bust", []byte("user:1")))

		assert.Equal(t, "user:1", receive(t, in))
	})

	t.Run("keeps topics apart", func(t *testing.T) {
		t.Parallel()
		bus := c.open(t, testkit.NewValkey(t))
		a, _ := subscribe(t, bus, "a")
		b, _ := subscribe(t, bus, "b")

		require.NoError(t, bus.Publish(t.Context(), "a", []byte("for a")))
		require.NoError(t, bus.Publish(t.Context(), "b", []byte("for b")))

		assert.Equal(t, "for a", receive(t, a))
		assert.Equal(t, "for b", receive(t, b))
		assert.Empty(t, a)
	})

	t.Run("stops one subscriber while others keep listening", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		bus := c.open(t, client)
		leaving, unsubscribe := subscribe(t, bus, "jobs")
		staying, _ := subscribe(t, bus, "jobs")

		unsubscribe()
		require.NoError(t, bus.Publish(t.Context(), "jobs", []byte("stop")))

		assert.Equal(t, "stop", receive(t, staying))
		assert.Empty(t, leaving)
		assert.Equal(t, int64(1), c.subscribers(t, client, "jobs"))
	})

	t.Run("unsubscribes on the server when the last subscriber leaves", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		bus := c.open(t, client)
		_, first := subscribe(t, bus, "jobs")
		_, second := subscribe(t, bus, "jobs")

		first()
		second()
		second()

		assert.Zero(t, c.subscribers(t, client, "jobs"))
	})

	t.Run("listens again after leaving a topic", func(t *testing.T) {
		t.Parallel()
		bus := c.open(t, testkit.NewValkey(t))
		_, unsubscribe := subscribe(t, bus, "jobs")
		unsubscribe()

		in, _ := subscribe(t, bus, "jobs")
		require.NoError(t, bus.Publish(t.Context(), "jobs", []byte("again")))

		assert.Equal(t, "again", receive(t, in))
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		bus := c.open(t, client)
		ctx, cancel := context.WithCancel(t.Context())
		in := make(received, 16)
		_, err := bus.Subscribe(ctx, "jobs", in.handle)
		require.NoError(t, err)

		cancel()

		assert.Eventually(t, func() bool { return c.subscribers(t, client, "jobs") == 0 },
			wait, 10*time.Millisecond)
	})
}

func openClient(t *testing.T, cfg redis.Config) *redis.Client {
	t.Helper()

	client, err := redis.Open(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	return client
}

func TestSubscribeFailures(t *testing.T) {
	t.Parallel()

	t.Run("gives up when the context has ended", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := newBus(t, testkit.NewValkey(t)).Subscribe(ctx, "jobs", func([]byte) {})

		require.ErrorIs(t, err, context.Canceled)
		require.ErrorContains(t, err, `subscribe "jobs"`)
	})

	t.Run("reports a closed client", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		bus := newBus(t, client)
		client.Close()

		_, err := bus.Subscribe(t.Context(), "jobs", func([]byte) {})
		require.ErrorContains(t, err, `subscribe "jobs"`)
		require.ErrorContains(t, bus.Publish(t.Context(), "jobs", nil), `publish "jobs"`)
	})

	t.Run("ends a subscription when the client closes", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		_, unsubscribe := subscribe(t, newBus(t, client), "jobs")

		client.Close()

		unsubscribe()
	})
}
