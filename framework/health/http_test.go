package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/health"
)

/*
serve sends a GET to the handler and returns the recorded response.
*/
func serve(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

	return recorder
}

func TestUp(t *testing.T) {
	t.Parallel()

	t.Run("answers ok without running any check", func(t *testing.T) {
		t.Parallel()

		response := serve(t, health.Up())

		assert.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"status":"up"}`, response.Body.String())
		assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	})
}

func TestReady(t *testing.T) {
	t.Parallel()

	t.Run("answers ok when every check passes or warns", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(
			returning("database", health.Result{Status: health.Pass}),
			returning("valkey", health.Result{Status: health.Warn, Detail: "slow"}),
		)

		response := serve(t, health.Ready(registry))

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
		assert.JSONEq(t, `{"status":"warn","checks":{"database":"pass","valkey":"warn"}}`, response.Body.String())
	})

	t.Run("answers unavailable naming the failed check without its detail", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(
			returning("database", health.Result{Status: health.Fail, Detail: "dial tcp 10.0.0.5:5432: refused"}),
			returning("valkey", health.Result{Status: health.Pass}),
		)

		response := serve(t, health.Ready(registry))

		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.JSONEq(t, `{"status":"fail","checks":{"database":"fail","valkey":"pass"}}`, response.Body.String())
	})

	t.Run("runs the checks under the request context", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(health.Func("context", func(ctx context.Context) health.Result {
			if ctx.Err() != nil {
				return health.Result{Status: health.Fail}
			}

			return health.Result{Status: health.Pass}
		}))

		response := serve(t, health.Ready(registry))

		assert.Equal(t, http.StatusOK, response.Code)
	})
}
