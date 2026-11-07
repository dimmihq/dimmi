package webpush_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
)

func TestDeriveKeys(t *testing.T) {
	t.Parallel()

	t.Run("derives the same valid pair from the same seed and another from another", func(t *testing.T) {
		t.Parallel()
		private, public, err := webpush.DeriveKeys([]byte("seed-one"))
		require.NoError(t, err)
		again, _, err := webpush.DeriveKeys([]byte("seed-one"))
		require.NoError(t, err)
		other, _, err := webpush.DeriveKeys([]byte("seed-two"))
		require.NoError(t, err)
		assert.Equal(t, private, again)
		assert.NotEqual(t, private, other)
		assert.Len(t, public, 87)
	})

	t.Run("refuses an empty seed", func(t *testing.T) {
		t.Parallel()
		_, _, err := webpush.DeriveKeys(nil)
		require.ErrorIs(t, err, webpush.ErrNoSeed)
	})

	t.Run("fills an empty configuration from the defaults", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{})
		require.NoError(t, err)
		factory := webpush.Factory(client, clock.System(), nil, webpush.Defaults{
			Seed: []byte("install"), Subject: "http://localhost:8080",
		})
		_, err = factory("webpush", config.Section{})
		require.NoError(t, err)

		factory = webpush.Factory(client, clock.System(), nil, webpush.Defaults{
			Seed: []byte("install"), Subject: "https://support.example.com",
		})
		_, err = factory("webpush", config.Section{})
		require.NoError(t, err)
	})
}
