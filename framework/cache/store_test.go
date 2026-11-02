package cache_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestValues(t *testing.T) {
	t.Parallel()

	t.Run("misses an absent key", func(t *testing.T) {
		t.Parallel()

		_, err := newStore(t).Get(t.Context(), "absent")

		require.ErrorIs(t, err, cache.ErrMiss)
	})

	t.Run("returns what was set, byte for byte", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		value := []byte{0, 1, 0xff, '{', '}'}

		require.NoError(t, s.Set(t.Context(), "user:{1}", value, long))

		got, err := s.Get(t.Context(), "user:{1}")
		require.NoError(t, err)
		assert.Equal(t, value, got)
	})

	t.Run("returns only present keys from many", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		require.NoError(t, s.Set(t.Context(), "a", []byte("1"), long))
		require.NoError(t, s.Set(t.Context(), "c", []byte("3"), long))

		got, err := s.Many(t.Context(), []string{"a", "b", "c"})
		require.NoError(t, err)
		assert.Equal(t, map[string][]byte{"a": []byte("1"), "c": []byte("3")}, got)

		empty, err := s.Many(t.Context(), nil)
		require.NoError(t, err)
		assert.Empty(t, empty)
	})

	t.Run("adds only when the key is absent", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)

		first, err := s.Add(t.Context(), "once", []byte("first"), long)
		require.NoError(t, err)
		second, err := s.Add(t.Context(), "once", []byte("second"), long)
		require.NoError(t, err)

		assert.True(t, first)
		assert.False(t, second)
		got, err := s.Get(t.Context(), "once")
		require.NoError(t, err)
		assert.Equal(t, "first", string(got))
	})

	t.Run("deletes a key, present or not", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		require.NoError(t, s.Set(t.Context(), "gone", []byte("x"), long))

		require.NoError(t, s.Delete(t.Context(), "gone"))
		require.NoError(t, s.Delete(t.Context(), "never-set"))

		_, err := s.Get(t.Context(), "gone")
		require.ErrorIs(t, err, cache.ErrMiss)
	})

	t.Run("rejects writes that would never expire", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)

		require.ErrorIs(t, s.Set(t.Context(), "k", nil, 0), cache.ErrTTLRequired)
		_, err := s.Add(t.Context(), "k", nil, -time.Second)
		require.ErrorIs(t, err, cache.ErrTTLRequired)
		_, err = s.Increment(t.Context(), "k", 1, 0)
		require.ErrorIs(t, err, cache.ErrTTLRequired)
		_, err = s.Remember(t.Context(), "k", 0, nil)
		require.ErrorIs(t, err, cache.ErrTTLRequired)
	})
}

func TestExpiry(t *testing.T) {
	t.Parallel()

	t.Run("drops values after their ttl", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		require.NoError(t, s.Set(t.Context(), "brief", []byte("x"), short))

		waitPastShortTTL()

		_, err := s.Get(t.Context(), "brief")
		require.ErrorIs(t, err, cache.ErrMiss)
	})

	t.Run("lets an add through again once the window closes", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		_, err := s.Add(t.Context(), "window", []byte("x"), short)
		require.NoError(t, err)

		waitPastShortTTL()

		added, err := s.Add(t.Context(), "window", []byte("y"), long)
		require.NoError(t, err)
		assert.True(t, added)
	})
}

func TestCounters(t *testing.T) {
	t.Parallel()

	t.Run("counts from zero", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)

		first, err := s.Increment(t.Context(), "hits", 1, long)
		require.NoError(t, err)
		second, err := s.Increment(t.Context(), "hits", 5, long)
		require.NoError(t, err)

		assert.Equal(t, []int64{1, 6}, []int64{first, second})
	})

	t.Run("keeps the first expiry while counting", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		_, err := s.Increment(t.Context(), "window", 1, short)
		require.NoError(t, err)
		_, err = s.Increment(t.Context(), "window", 1, long)
		require.NoError(t, err)

		waitPastShortTTL()

		restarted, err := s.Increment(t.Context(), "window", 1, long)
		require.NoError(t, err)
		assert.Equal(t, int64(1), restarted)
	})

	t.Run("refuses to count a value that is not a number", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		require.NoError(t, s.Set(t.Context(), "name", []byte("ada"), long))

		_, err := s.Increment(t.Context(), "name", 1, long)

		require.ErrorIs(t, err, cache.ErrNotInteger)
	})
}

func TestFlush(t *testing.T) {
	t.Parallel()

	t.Run("removes every key in the store, past one scan batch", func(t *testing.T) {
		t.Parallel()
		s := newStore(t)
		for i := range 1200 {
			require.NoError(t, s.Set(t.Context(), fmt.Sprint("k", i), []byte("v"), long))
		}

		require.NoError(t, s.Flush(t.Context()))

		got, err := s.Many(t.Context(), []string{"k0", "k600", "k1199"})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("leaves other keyspaces alone", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		sessions := openStore(t, client, "sessions")
		pages := openStore(t, client, "pages")
		require.NoError(t, sessions.Set(t.Context(), "s1", []byte("x"), long))
		require.NoError(t, pages.Set(t.Context(), "p1", []byte("y"), long))

		require.NoError(t, pages.Flush(t.Context()))

		_, err := sessions.Get(t.Context(), "s1")
		require.NoError(t, err)
		_, err = pages.Get(t.Context(), "p1")
		require.ErrorIs(t, err, cache.ErrMiss)
	})
}

func TestServerErrors(t *testing.T) {
	t.Parallel()
	client := testkit.NewValkey(t)
	s := openStore(t, client, "cache")
	client.Close()
	ctx := t.Context()

	_, err := s.Get(ctx, "k")
	require.ErrorContains(t, err, `get "k"`)
	_, err = s.Many(ctx, []string{"k"})
	require.ErrorContains(t, err, "get many")
	require.ErrorContains(t, s.Set(ctx, "k", nil, long), `set "k"`)
	_, err = s.Add(ctx, "k", nil, long)
	require.ErrorContains(t, err, `add "k"`)
	_, err = s.Increment(ctx, "k", 1, long)
	require.ErrorContains(t, err, `increment "k"`)
	require.ErrorContains(t, s.Delete(ctx, "k"), `delete "k"`)
	require.ErrorContains(t, s.Flush(ctx), "flush scan")
}
