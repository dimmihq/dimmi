package ratelimit_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
Test windows. The short one leaves room for a few calls in a row even on
a loaded CI machine running with -race.
*/
const (
	long  = time.Minute
	short = 300 * time.Millisecond
)

func newLimiter(t *testing.T) *ratelimit.Limiter {
	t.Helper()

	return openLimiter(t, testkit.NewValkey(t))
}

func openLimiter(t *testing.T, client *redis.Client) *ratelimit.Limiter {
	t.Helper()

	limiter, err := ratelimit.New(client, ratelimit.Options{Keyspace: "rl"})
	require.NoError(t, err)

	return limiter
}

func waitPastShortWindow() {
	clock.System().Sleep(2 * short)
}

func TestAttempt(t *testing.T) {
	t.Parallel()

	t.Run("allows up to the maximum, then refuses", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)

		for want := 2; want >= 0; want-- {
			got, err := l.Attempt(t.Context(), "login", 3, long)
			require.NoError(t, err)
			assert.Equal(t, ratelimit.Result{Allowed: true, Limit: 3, Remaining: want}, got)
		}

		refused, err := l.Attempt(t.Context(), "login", 3, long)
		require.NoError(t, err)
		assert.False(t, refused.Allowed)
		assert.Zero(t, refused.Remaining)
		assert.Positive(t, refused.RetryAfter)
		assert.LessOrEqual(t, refused.RetryAfter, long)
	})

	t.Run("allows again once the window resets", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		_, err := l.Attempt(t.Context(), "send", 1, short)
		require.NoError(t, err)
		refused, err := l.Attempt(t.Context(), "send", 1, short)
		require.NoError(t, err)
		require.False(t, refused.Allowed)

		waitPastShortWindow()

		again, err := l.Attempt(t.Context(), "send", 1, short)
		require.NoError(t, err)
		assert.True(t, again.Allowed)
	})

	t.Run("does not count refused attempts", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		for range 5 {
			_, err := l.Attempt(t.Context(), "k", 2, long)
			require.NoError(t, err)
		}

		count, err := l.Hit(t.Context(), "k", 3, long)
		require.NoError(t, err)

		assert.True(t, count.Allowed, "two counted attempts plus this hit stay within three")
	})

	t.Run("keeps keys apart", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		_, err := l.Attempt(t.Context(), "ip:1", 1, long)
		require.NoError(t, err)

		other, err := l.Attempt(t.Context(), "ip:2", 1, long)
		require.NoError(t, err)

		assert.True(t, other.Allowed)
	})

	t.Run("stores counters in its keyspace", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		_, err := openLimiter(t, client).Attempt(t.Context(), "login", 1, long)
		require.NoError(t, err)

		rdb := client.Rueidis()
		exists, err := rdb.Do(t.Context(), rdb.B().Exists().Key(client.Key("rl", "login")).Build()).AsInt64()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)
	})
}

func TestHit(t *testing.T) {
	t.Parallel()

	t.Run("counts past the limit", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		for range 3 {
			_, err := l.Hit(t.Context(), "failed", 2, long)
			require.NoError(t, err)
		}

		remaining, err := l.Remaining(t.Context(), "failed", 3, long)
		require.NoError(t, err)

		assert.Zero(t, remaining, "three hits use up a limit of three")
	})

	t.Run("refuses the hit that crosses the limit", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		first, err := l.Hit(t.Context(), "failed", 1, long)
		require.NoError(t, err)
		second, err := l.Hit(t.Context(), "failed", 1, long)
		require.NoError(t, err)

		assert.True(t, first.Allowed)
		assert.False(t, second.Allowed)
		assert.Positive(t, second.RetryAfter)
	})
}

func TestInspect(t *testing.T) {
	t.Parallel()

	t.Run("reports a fresh key as fully available", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)

		remaining, err := l.Remaining(t.Context(), "fresh", 5, long)
		require.NoError(t, err)
		wait, err := l.AvailableIn(t.Context(), "fresh", 5, long)
		require.NoError(t, err)

		assert.Equal(t, 5, remaining)
		assert.Zero(t, wait)
	})

	t.Run("reports when a used-up key frees", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		_, err := l.Attempt(t.Context(), "used", 1, long)
		require.NoError(t, err)

		remaining, err := l.Remaining(t.Context(), "used", 1, long)
		require.NoError(t, err)
		wait, err := l.AvailableIn(t.Context(), "used", 1, long)
		require.NoError(t, err)

		assert.Zero(t, remaining)
		assert.Positive(t, wait)
		assert.LessOrEqual(t, wait, long)
	})

	t.Run("counts nothing while inspecting", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		for range 3 {
			_, err := l.Remaining(t.Context(), "k", 1, long)
			require.NoError(t, err)
		}

		got, err := l.Attempt(t.Context(), "k", 1, long)
		require.NoError(t, err)

		assert.True(t, got.Allowed)
	})
}

func TestClear(t *testing.T) {
	t.Parallel()

	t.Run("resets a used-up key", func(t *testing.T) {
		t.Parallel()
		l := newLimiter(t)
		_, err := l.Attempt(t.Context(), "login", 1, long)
		require.NoError(t, err)

		require.NoError(t, l.Clear(t.Context(), "login"))

		got, err := l.Attempt(t.Context(), "login", 1, long)
		require.NoError(t, err)
		assert.True(t, got.Allowed)
	})

	t.Run("clears an absent key without error", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, newLimiter(t).Clear(t.Context(), "never-used"))
	})
}

func TestInvalidLimits(t *testing.T) {
	t.Parallel()
	l := newLimiter(t)
	ctx := t.Context()

	_, err := l.Attempt(ctx, "k", 0, long)
	require.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
	_, err = l.Hit(ctx, "k", 1, 0)
	require.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
	_, err = l.Remaining(ctx, "k", -1, long)
	require.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
	_, err = l.AvailableIn(ctx, "k", 1, -time.Second)
	require.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
}

func TestUnavailable(t *testing.T) {
	t.Parallel()
	client := testkit.NewValkey(t)
	l := openLimiter(t, client)
	client.Close()
	ctx := t.Context()

	_, err := l.Attempt(ctx, "k", 1, long)
	require.ErrorIs(t, err, ratelimit.ErrUnavailable)
	require.ErrorContains(t, err, `attempt "k"`)
	_, err = l.Hit(ctx, "k", 1, long)
	require.ErrorIs(t, err, ratelimit.ErrUnavailable)
	_, err = l.Remaining(ctx, "k", 1, long)
	require.ErrorIs(t, err, ratelimit.ErrUnavailable)
	_, err = l.AvailableIn(ctx, "k", 1, long)
	require.ErrorIs(t, err, ratelimit.ErrUnavailable)
	require.ErrorIs(t, l.Clear(ctx, "k"), ratelimit.ErrUnavailable)
}

func TestWriteHeaders(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)

	t.Run("states the limit and what remains", func(t *testing.T) {
		t.Parallel()
		h := http.Header{}

		ratelimit.Result{Allowed: true, Limit: 60, Remaining: 59}.WriteHeaders(h, now)

		assert.Equal(t, http.Header{
			"X-Ratelimit-Limit":     {"60"},
			"X-Ratelimit-Remaining": {"59"},
		}, h)
	})

	t.Run("tells a refused client when to come back", func(t *testing.T) {
		t.Parallel()
		h := http.Header{}

		ratelimit.Result{Limit: 60, RetryAfter: 1500 * time.Millisecond}.WriteHeaders(h, now)

		assert.Equal(t, "0", h.Get("X-RateLimit-Remaining"))
		assert.Equal(t, "2", h.Get("Retry-After"))
		assert.Equal(t, "1700000002", h.Get("X-RateLimit-Reset"))
	})

	t.Run("never asks to retry in zero seconds", func(t *testing.T) {
		t.Parallel()
		h := http.Header{}

		ratelimit.Result{Limit: 1}.WriteHeaders(h, now)

		assert.Equal(t, "1", h.Get("Retry-After"))
	})
}
