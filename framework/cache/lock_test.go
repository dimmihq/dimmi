package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
)

func TestLocks(t *testing.T) {
	t.Parallel()

	t.Run("lets one holder at a time", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)

		held, release, err := s.TryLock(t.Context(), "report")
		require.NoError(t, err)
		require.NoError(t, held.Err())

		_, _, err = s.TryLock(t.Context(), "report")
		require.ErrorIs(t, err, cache.ErrLocked)

		release()

		_, again, err := s.TryLock(t.Context(), "report")
		require.NoError(t, err)
		again()
	})

	t.Run("waits until the holder releases", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		_, release, err := s.TryLock(t.Context(), "report")
		require.NoError(t, err)

		go func() {
			waitPastShortTTL()
			release()
		}()

		_, waited, err := s.Lock(t.Context(), "report")
		require.NoError(t, err)
		waited()
	})

	t.Run("stops waiting when the context ends", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		_, release, err := s.TryLock(t.Context(), "report")
		require.NoError(t, err)
		defer release()

		ctx, cancel := context.WithTimeout(t.Context(), short)
		defer cancel()

		_, _, err = s.Lock(ctx, "report")
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("ends the held context on release", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		held, release, err := s.Lock(t.Context(), "report")
		require.NoError(t, err)

		release()

		select {
		case <-held.Done():
		case <-clock.System().After(5 * time.Second):
			assert.Fail(t, "held context still alive after release")
		}
	})
}
