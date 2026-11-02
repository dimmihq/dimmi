package telemetry_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
newPrometheus returns telemetry with prometheus metrics, shut down when
the test ends.
*/
func newPrometheus(t *testing.T) *telemetry.Telemetry {
	t.Helper()
	cfg := off()
	cfg.Metrics = "prometheus"

	tel, err := telemetry.New(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, tel.Shutdown(t.Context())) })

	return tel
}

/*
scrape returns what a Prometheus scrape of the telemetry sees.
*/
func scrape(t *testing.T, tel *telemetry.Telemetry) string {
	t.Helper()
	response := httptest.NewRecorder()

	tel.MetricsHandler().ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, response.Code)

	return response.Body.String()
}

func TestMetrics(t *testing.T) {
	t.Parallel()

	t.Run("serves every instrument to prometheus", func(t *testing.T) {
		t.Parallel()
		tel := newPrometheus(t)
		ctx := t.Context()

		counter, err := tel.Counter("jobs_processed")
		require.NoError(t, err)
		histogram, err := tel.Histogram("job_seconds", 0.1, 0.5)
		require.NoError(t, err)
		gauge, err := tel.Gauge("queue_depth")
		require.NoError(t, err)

		counter.Add(ctx, 2, telemetry.String("queue", "default"))
		counter.Add(ctx, 1, telemetry.String("queue", "default"))
		histogram.Record(ctx, 0.3)
		gauge.Set(ctx, 4, telemetry.String("queue", "default"))

		body := scrape(t, tel)
		for _, want := range []string{
			`jobs_processed_total{queue="default"} 3`,
			`job_seconds_bucket{le="0.1"} 0`,
			`job_seconds_bucket{le="0.5"} 1`,
			`job_seconds_count 1`,
			`queue_depth{queue="default"} 4`,
			`service_name="shop"`,
			`go_goroutines`,
		} {
			assert.Contains(t, body, want)
		}
	})

	t.Run("keeps each instance's metrics apart", func(t *testing.T) {
		t.Parallel()
		first, second := newPrometheus(t), newPrometheus(t)

		counter, err := first.Counter("signups")
		require.NoError(t, err)
		counter.Add(t.Context(), 1)

		assert.Contains(t, scrape(t, first), "signups_total 1")
		assert.NotContains(t, scrape(t, second), "signups_total")
	})

	t.Run("rejects invalid instrument names", func(t *testing.T) {
		t.Parallel()
		tel := newPrometheus(t)

		_, err := tel.Counter("1orders")
		require.ErrorContains(t, err, "create counter 1orders")

		_, err = tel.Histogram("2latency")
		require.ErrorContains(t, err, "create histogram 2latency")

		_, err = tel.Gauge("3depth")
		require.ErrorContains(t, err, "create gauge 3depth")
	})
}
