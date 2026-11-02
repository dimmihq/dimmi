package cache_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
)

func TestRemember(t *testing.T) {
	t.Parallel()

	t.Run("loads once and serves from cache after", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		var loads atomic.Int32
		load := func(context.Context) ([]byte, error) { loads.Add(1); return []byte("report"), nil }

		first, err := s.Remember(t.Context(), "report", long, load)
		require.NoError(t, err)
		second, err := s.Remember(t.Context(), "report", long, load)
		require.NoError(t, err)

		assert.Equal(t, "report", string(first))
		assert.Equal(t, first, second)
		assert.Equal(t, int32(1), loads.Load())
	})

	t.Run("lets one of many concurrent callers load", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		var loads atomic.Int32
		load := cache.Loader(func(context.Context) ([]byte, error) {
			loads.Add(1)
			waitPastShortTTL()

			return []byte("slow"), nil
		})

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				got, err := s.Remember(t.Context(), "slow", long, load)
				assert.NoError(t, err)
				assert.Equal(t, "slow", string(got))
			})
		}
		wg.Wait()

		assert.Equal(t, int32(1), loads.Load())
	})

	t.Run("caches nothing when loading fails", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		errLoad := errors.New("upstream down")

		_, err := s.Remember(t.Context(), "k", long, func(context.Context) ([]byte, error) { return nil, errLoad })
		require.ErrorIs(t, err, errLoad)

		got, err := s.Remember(t.Context(), "k", long, func(context.Context) ([]byte, error) {
			return []byte("ok"), nil
		})
		require.NoError(t, err)
		assert.Equal(t, "ok", string(got))
	})

	t.Run("loads again after forget", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		version := 0
		load := func(context.Context) ([]byte, error) { version++; return []byte(strconv.Itoa(version)), nil }
		_, err := s.Remember(t.Context(), "k", long, load)
		require.NoError(t, err)

		require.NoError(t, s.Forget(t.Context(), "k"))

		got, err := s.Remember(t.Context(), "k", long, load)
		require.NoError(t, err)
		assert.Equal(t, "2", string(got))
	})
}

func TestTyped(t *testing.T) {
	t.Parallel()

	type user struct{ Name string }

	t.Run("round-trips values as json", func(t *testing.T) {
		t.Parallel()
		users := cache.For[user](newStore(t), long)

		got, err := users.Remember(t.Context(), "user:1", func(context.Context) (user, error) {
			return user{Name: "Ada"}, nil
		})

		require.NoError(t, err)
		assert.Equal(t, user{Name: "Ada"}, got)
	})

	t.Run("returns the loader's error", func(t *testing.T) {
		t.Parallel()
		errLoad := errors.New("load")

		_, err := cache.For[user](newStore(t), long).Remember(t.Context(), "user:1",
			func(context.Context) (user, error) { return user{}, errLoad })

		require.ErrorIs(t, err, errLoad)
	})

	t.Run("reports a cached value of another shape", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		_, err := cache.For[string](s, long).Remember(t.Context(), "k",
			func(context.Context) (string, error) { return "text", nil })
		require.NoError(t, err)

		_, err = cache.For[user](s, long).Remember(t.Context(), "k",
			func(context.Context) (user, error) { return user{}, nil })

		require.ErrorContains(t, err, `decode cached "k"`)
	})
}
