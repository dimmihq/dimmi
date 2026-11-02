package ratelimit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/ratelimit"
)

func newLocal(t *testing.T, interval time.Duration, burst int) (*ratelimit.Local, *clock.Fake) {
	t.Helper()

	fake := clock.NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	local, err := ratelimit.NewLocal(interval, burst, fake)
	require.NoError(t, err)

	return local, fake
}

func TestLocalAllow(t *testing.T) {
	t.Parallel()

	t.Run("allows a burst, then one per interval", func(t *testing.T) {
		t.Parallel()
		local, fake := newLocal(t, 2*time.Second, 2)

		assert.True(t, local.Allow("host"))
		assert.True(t, local.Allow("host"))
		assert.False(t, local.Allow("host"))

		fake.Travel(2 * time.Second)

		assert.True(t, local.Allow("host"))
		assert.False(t, local.Allow("host"))
	})

	t.Run("paces keys apart", func(t *testing.T) {
		t.Parallel()
		local, _ := newLocal(t, time.Second, 1)

		assert.True(t, local.Allow("a.example"))
		assert.True(t, local.Allow("b.example"))
		assert.False(t, local.Allow("a.example"))
	})

	t.Run("rejects a rate that is not positive", func(t *testing.T) {
		t.Parallel()

		_, err := ratelimit.NewLocal(0, 1, clock.System())
		require.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
		_, err = ratelimit.NewLocal(time.Second, 0, clock.System())
		require.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
	})
}

func TestLocalWait(t *testing.T) {
	t.Parallel()

	t.Run("returns at once while tokens remain", func(t *testing.T) {
		t.Parallel()
		local, _ := newLocal(t, time.Second, 1)

		require.NoError(t, local.Wait(t.Context(), "host"))
	})

	t.Run("waits for the next token", func(t *testing.T) {
		t.Parallel()
		local, fake := newLocal(t, time.Second, 1)
		require.NoError(t, local.Wait(t.Context(), "host"))

		done := make(chan error, 1)
		go func() { done <- local.Wait(t.Context(), "host") }()
		require.NoError(t, fake.BlockUntilContext(t.Context(), 1))
		fake.Travel(time.Second)

		require.NoError(t, <-done)
	})

	t.Run("gives the token back when the context ends", func(t *testing.T) {
		t.Parallel()
		local, fake := newLocal(t, time.Second, 1)
		require.NoError(t, local.Wait(t.Context(), "host"))
		ctx, cancel := context.WithCancel(t.Context())

		done := make(chan error, 1)
		go func() { done <- local.Wait(ctx, "host") }()
		require.NoError(t, fake.BlockUntilContext(t.Context(), 1))
		cancel()

		err := <-done
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorContains(t, err, `wait "host"`)
		fake.Travel(time.Second)
		assert.True(t, local.Allow("host"), "the canceled wait no longer holds the token")
	})
}

func TestLocalEviction(t *testing.T) {
	t.Parallel()

	t.Run("drops keys whose bucket has refilled", func(t *testing.T) {
		t.Parallel()
		local, fake := newLocal(t, time.Second, 2)
		for i := range 100 {
			local.Allow(fmt.Sprint("host", i))
		}
		require.Equal(t, 100, ratelimit.Buckets(local))

		fake.Travel(2 * time.Second)
		local.Allow("fresh")

		assert.Equal(t, 1, ratelimit.Buckets(local))
	})

	t.Run("keeps keys that are still pacing", func(t *testing.T) {
		t.Parallel()
		local, fake := newLocal(t, time.Minute, 1)
		local.Allow("slow")

		fake.Travel(30 * time.Second)
		local.Allow("other")
		fake.Travel(31 * time.Second)
		local.Allow("other")

		assert.Equal(t, 1, ratelimit.Buckets(local), "only the key used in this minute stays")
		assert.False(t, local.Allow("other"))
	})
}
