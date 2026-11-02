package middleware_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func newLimiter(t *testing.T, client *redis.Client) *ratelimit.Limiter {
	t.Helper()

	l, err := ratelimit.New(client, ratelimit.Options{Keyspace: "rl"})
	require.NoError(t, err)

	return l
}

/*
closedLimiter returns a limiter whose connection is already closed, so
every attempt fails the way it does while Valkey is down.
*/
func closedLimiter(t *testing.T) *ratelimit.Limiter {
	t.Helper()

	client, err := redis.Open(t.Context(), testkit.ValkeyConfig(t))
	require.NoError(t, err)
	client.Close()

	return newLimiter(t, client)
}

func TestThrottle(t *testing.T) {
	t.Parallel()

	t.Run("allows up to the limit with headers, then refuses", func(t *testing.T) {
		t.Parallel()
		h := middleware.Throttle(newLimiter(t, testkit.NewValkey(t)), middleware.ThrottleOptions{
			Name: "api", MaxAttempts: 2, Decay: time.Minute,
		})(echo())

		first := serve(h, request(t, http.MethodGet, "/", ""))
		assert.Equal(t, http.StatusOK, first.Code)
		assert.Equal(t, "2", first.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "1", first.Header().Get("X-RateLimit-Remaining"))
		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodGet, "/", "")).Code)

		refused := serve(h, request(t, http.MethodGet, "/", ""))
		assert.Equal(t, http.StatusTooManyRequests, refused.Code)
		assert.NotEmpty(t, refused.Header().Get("Retry-After"))
		assert.Equal(t, "too_many_requests", code(t, refused.Body))
	})

	t.Run("counts per key", func(t *testing.T) {
		t.Parallel()
		h := middleware.Throttle(newLimiter(t, testkit.NewValkey(t)), middleware.ThrottleOptions{
			Name: "login", MaxAttempts: 1, Decay: time.Minute,
			Key: func(r *http.Request) string { return r.URL.Query().Get("email") },
		})(echo())

		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodPost, "/?email=a", "")).Code)
		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodPost, "/?email=b", "")).Code)
		assert.Equal(t, http.StatusTooManyRequests, serve(h, request(t, http.MethodPost, "/?email=a", "")).Code)
	})

	t.Run("fails closed while counts cannot be checked", func(t *testing.T) {
		t.Parallel()
		h := middleware.Throttle(closedLimiter(t), middleware.ThrottleOptions{
			Name: "login", MaxAttempts: 1, Decay: time.Minute,
		})(echo())
		rec := serve(h, request(t, http.MethodPost, "/", ""))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "service_unavailable", code(t, rec.Body))
	})

	t.Run("fails open when asked to", func(t *testing.T) {
		t.Parallel()
		h := middleware.Throttle(closedLimiter(t), middleware.ThrottleOptions{
			Name: "api", MaxAttempts: 1, Decay: time.Minute, FailOpen: true,
		})(echo())

		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodGet, "/", "")).Code)
	})
}
