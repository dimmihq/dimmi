package otlp_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/telemetry"
	"github.com/dimmihq/dimmi/framework/telemetry/otlp"
)

func TestTracing(t *testing.T) {
	t.Parallel()

	t.Run("sends spans to the collector with the configured headers", func(t *testing.T) {
		t.Parallel()

		var (
			mu     sync.Mutex
			bodies []string
			teams  []string
		)

		collector := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			mu.Lock()
			bodies = append(bodies, r.URL.Path+" "+string(body))
			teams = append(teams, r.Header.Get("x-team"))
			mu.Unlock()
		}))
		t.Cleanup(collector.Close)

		cfg := telemetry.Config{
			ServiceName: "shop", Tracing: otlp.Driver, Metrics: "none",
			OTLPEndpoint: collector.URL + "/v1/traces", OTLPHeaders: map[string]string{"x-team": "alpha"},
		}
		tel, err := telemetry.New(t.Context(), cfg, otlp.Tracing())
		require.NoError(t, err)

		_, span := tel.Start(t.Context(), "checkout")
		span.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, bodies, 1)
		assert.Contains(t, bodies[0], "/v1/traces")
		assert.Contains(t, bodies[0], "checkout")
		assert.Equal(t, []string{"alpha"}, teams)
	})

	t.Run("reports an exporter that cannot start", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := telemetry.New(ctx, telemetry.Config{Tracing: otlp.Driver, Metrics: "none"}, otlp.Tracing())

		require.ErrorContains(t, err, "create otlp exporter: otlp exporter")
	})
}
