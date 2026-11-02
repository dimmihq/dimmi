package redis_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("connects in every single-server mode", func(t *testing.T) {
		t.Parallel()

		for _, mode := range []redis.Mode{redis.ModeAuto, redis.ModeStandalone} {
			cfg := testkit.ValkeyConfig(t)
			cfg.Mode = mode

			client, err := redis.Open(t.Context(), cfg)
			require.NoError(t, err, mode)
			require.NoError(t, client.Ping(t.Context()))
			client.Close()
		}
	})

	t.Run("rejects bad configuration before connecting", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			cfg  redis.Config
			want error
		}{
			{
				name: "unknown mode",
				cfg:  redis.Config{URL: "redis://localhost:6379", Mode: "ring"},
				want: redis.ErrInvalidMode,
			},
			{
				name: "sentinel without master",
				cfg:  redis.Config{URL: "redis://localhost:26379", Mode: redis.ModeSentinel},
				want: redis.ErrSentinelMaster,
			},
			{
				name: "cluster with a database",
				cfg:  redis.Config{URL: "redis://localhost:6379/2", Mode: redis.ModeCluster},
				want: redis.ErrClusterDatabase,
			},
		}
		for _, tc := range cases {
			_, err := redis.Open(t.Context(), tc.cfg)
			require.ErrorIs(t, err, tc.want, tc.name)
		}
	})

	t.Run("refuses cluster mode against a single server", func(t *testing.T) {
		t.Parallel()
		cfg := testkit.ValkeyConfig(t)
		cfg.Mode = redis.ModeCluster

		_, err := redis.Open(t.Context(), cfg)

		require.ErrorIs(t, err, redis.ErrNotCluster)
	})

	t.Run("rejects a malformed url", func(t *testing.T) {
		t.Parallel()

		_, err := redis.Open(t.Context(), redis.Config{URL: "http://localhost", Mode: redis.ModeAuto})

		require.ErrorContains(t, err, "parse valkey url")
	})

	t.Run("fails when the server cannot be reached", func(t *testing.T) {
		t.Parallel()

		_, err := redis.Open(t.Context(), redis.Config{URL: "redis://127.0.0.1:1", Mode: redis.ModeStandalone})

		require.ErrorContains(t, err, "connect to valkey")
	})

	t.Run("asks sentinels for the primary", func(t *testing.T) {
		t.Parallel()

		_, err := redis.Open(t.Context(), redis.Config{
			URL:      testkit.Env(t, "REDIS_URL"),
			Mode:     redis.ModeSentinel,
			Sentinel: redis.SentinelConfig{Master: "primary"},
		})

		require.Error(t, err, "a plain server is not a sentinel")
	})
}

func TestKeys(t *testing.T) {
	t.Parallel()
	client := testkit.NewValkey(t)
	prefix := client.Prefix("")[:len(client.Prefix(""))-1]

	assert.Equal(t, prefix+"cache:{user:42}", client.Key("cache", "user:42"))
	assert.Equal(t, prefix+"rl:{login}:1717000000:ip", client.Key("rl", "login", "1717000000", "ip"))
	assert.Equal(t, prefix+"cache:", client.Prefix("cache"))
	assert.NotNil(t, client.Rueidis())
	assert.NotEmpty(t, client.Option().InitAddress)
}

func TestPingAfterClose(t *testing.T) {
	t.Parallel()
	client, err := redis.Open(t.Context(), testkit.ValkeyConfig(t))
	require.NoError(t, err)

	client.Close()

	require.ErrorContains(t, client.Ping(t.Context()), "ping valkey")
}
