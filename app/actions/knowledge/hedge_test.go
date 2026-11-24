package knowledge

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHedged(t *testing.T) {
	t.Parallel()

	t.Run("a quick call is made once", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		got, err := hedged(context.Background(), time.Second, func(context.Context) (string, error) {
			calls.Add(1)
			return "vector", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "vector", got)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("a slow call is raced by a second, and the faster answer wins", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		got, err := hedged(context.Background(), 20*time.Millisecond, func(ctx context.Context) (string, error) {
			if calls.Add(1) == 1 {
				<-ctx.Done()
				return "", ctx.Err()
			}
			return "fast", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "fast", got, "the first call never answers; the search did not wait for it")
		assert.Equal(t, int32(2), calls.Load())
	})

	t.Run("a call that fails is tried again at once", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		got, err := hedged(context.Background(), time.Hour, func(context.Context) (string, error) {
			if calls.Add(1) == 1 {
				return "", errors.New("connection reset")
			}
			return "second", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "second", got)
	})

	t.Run("two failures fail with the first error", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		down := errors.New("embedder is down")
		_, err := hedged(context.Background(), time.Hour, func(context.Context) (string, error) {
			if calls.Add(1) == 1 {
				return "", down
			}
			return "", errors.New("still down")
		})
		require.ErrorIs(t, err, down)
		assert.Equal(t, int32(2), calls.Load(), "no third call is made")
	})

	t.Run("no wait means one call and no second", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		down := errors.New("embedder is down")
		_, err := hedged(context.Background(), 0, func(context.Context) (string, error) {
			calls.Add(1)
			return "", down
		})
		require.ErrorIs(t, err, down)
		assert.Equal(t, int32(1), calls.Load())
	})
}
