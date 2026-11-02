package telemetry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/dimmihq/dimmi/framework/telemetry"
	"github.com/dimmihq/dimmi/framework/telemetry/otlp"
)

/*
off is a configuration with every part switched off.
*/
func off() telemetry.Config {
	return telemetry.Config{ServiceName: "shop", Tracing: "none", Metrics: "none"}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("records nothing with every part off", func(t *testing.T) {
		t.Parallel()
		tel, err := telemetry.New(t.Context(), off())
		require.NoError(t, err)

		ctx, span := tel.Start(t.Context(), "checkout", telemetry.String("cart_id", "c1"))
		span.Set(telemetry.Bool("paid", true))
		span.Fail(errors.New("card declined"))
		span.End()
		tel.Report(ctx, errors.New("card declined"))
		tel.Report(ctx, nil)

		counter, err := tel.Counter("orders")
		require.NoError(t, err)
		counter.Add(ctx, 1)

		response := httptest.NewRecorder()
		tel.MetricsHandler().ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodGet, "/metrics", nil))

		assert.Equal(t, http.StatusNotFound, response.Code)
		require.NoError(t, tel.Shutdown(t.Context()))
	})

	t.Run("rejects an unknown tracing driver", func(t *testing.T) {
		t.Parallel()
		cfg := off()
		cfg.Tracing = "zipkin"

		_, err := telemetry.New(t.Context(), cfg)

		require.ErrorIs(t, err, telemetry.ErrUnknownDriver)
		require.ErrorContains(t, err, `tracing driver "zipkin"`)
	})

	t.Run("needs the application to register a tracing exporter", func(t *testing.T) {
		t.Parallel()
		cfg := off()
		cfg.Tracing = "otlp"

		_, err := telemetry.New(t.Context(), cfg)

		require.ErrorIs(t, err, telemetry.ErrUnknownDriver)
	})

	t.Run("reports an exporter that cannot start", func(t *testing.T) {
		t.Parallel()
		cfg := off()
		cfg.Tracing = "broken"
		failing := func(context.Context, telemetry.Config) (sdktrace.SpanExporter, error) {
			return nil, errors.New("no route to collector")
		}
		broken := telemetry.WithExporter("broken", failing)

		_, err := telemetry.New(t.Context(), cfg, broken)

		require.ErrorContains(t, err, "create broken exporter: no route to collector")
	})

	t.Run("rejects an unknown metrics driver", func(t *testing.T) {
		t.Parallel()
		cfg := off()
		cfg.Metrics = "statsd"

		_, err := telemetry.New(t.Context(), cfg)

		require.ErrorIs(t, err, telemetry.ErrUnknownDriver)
		require.ErrorContains(t, err, `metrics driver "statsd"`)
	})

	t.Run("rejects a malformed sentry dsn", func(t *testing.T) {
		t.Parallel()
		cfg := off()
		cfg.SentryDSN = "not a dsn"

		_, err := telemetry.New(t.Context(), cfg)

		require.ErrorContains(t, err, "create sentry client")
	})

	t.Run("reports a shutdown that runs out of time", func(t *testing.T) {
		t.Parallel()
		backend := newCollector(t)
		cfg := off()
		cfg.Tracing = "otlp"
		cfg.OTLPEndpoint = backend.server.URL + "/v1/traces"
		tel, err := telemetry.New(t.Context(), cfg, otlp.Tracing())
		require.NoError(t, err)
		_, span := tel.Start(t.Context(), "checkout")
		span.End()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorContains(t, tel.Shutdown(ctx), "shut down telemetry")
	})
}

func TestTracing(t *testing.T) {
	t.Parallel()

	t.Run("exports spans over otlp http", func(t *testing.T) {
		t.Parallel()
		backend := newCollector(t)
		cfg := off()
		cfg.Tracing = "otlp"
		cfg.OTLPEndpoint = backend.server.URL + "/v1/traces"
		cfg.OTLPHeaders = map[string]string{"x-team": "alpha"}
		tel, err := telemetry.New(t.Context(), cfg, otlp.Tracing())
		require.NoError(t, err)

		ctx, parent := tel.Start(t.Context(), "checkout", telemetry.String("cart_id", "c1"))
		_, child := tel.Start(ctx, "charge_card", telemetry.Int("attempt", 1))
		child.Fail(errors.New("card declined"))
		child.End()
		parent.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		requests := backend.received()
		require.Len(t, requests, 1)
		assert.Equal(t, "/v1/traces", requests[0].path)
		assert.Equal(t, "alpha", requests[0].header.Get("x-team"))
		for _, want := range []string{"shop", "checkout", "charge_card", "cart_id", "card declined"} {
			assert.Contains(t, requests[0].body, want)
		}
	})

	t.Run("hands its providers to libraries that trace on their own", func(t *testing.T) {
		t.Parallel()
		backend := newCollector(t)
		cfg := off()
		cfg.Tracing = "otlp"
		cfg.Metrics = "prometheus"
		cfg.OTLPEndpoint = backend.server.URL + "/v1/traces"
		tel, err := telemetry.New(t.Context(), cfg, otlp.Tracing())
		require.NoError(t, err)

		_, span := tel.TracerProvider().Tracer("library").Start(t.Context(), "library_query")
		span.End()
		counter, err := tel.MeterProvider().Meter("library").Int64Counter("library_calls")
		require.NoError(t, err)
		counter.Add(t.Context(), 1)

		response := httptest.NewRecorder()
		scrape := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
		tel.MetricsHandler().ServeHTTP(response, scrape)
		require.NoError(t, tel.Shutdown(t.Context()))

		assert.Contains(t, response.Body.String(), "library_calls_total")
		require.Len(t, backend.received(), 1)
		assert.Contains(t, backend.received()[0].body, "library_query")
	})
}

func TestReport(t *testing.T) {
	t.Parallel()

	t.Run("sends errors to sentry tagged with the trace", func(t *testing.T) {
		t.Parallel()
		sentry := newCollector(t)
		traces := newCollector(t)
		cfg := off()
		cfg.Environment = "staging"
		cfg.Tracing = "otlp"
		cfg.OTLPEndpoint = traces.server.URL + "/v1/traces"
		cfg.SentryDSN = strings.Replace(sentry.server.URL, "http://", "http://public@", 1) + "/42"
		tel, err := telemetry.New(t.Context(), cfg, otlp.Tracing())
		require.NoError(t, err)

		ctx, span := tel.Start(t.Context(), "nightly_export")
		tel.Report(ctx, errors.New("export bucket missing"))
		span.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		requests := sentry.received()
		require.Len(t, requests, 1)
		assert.Equal(t, "/api/42/envelope/", requests[0].path)
		for _, want := range []string{"export bucket missing", "staging", "trace_id"} {
			assert.Contains(t, requests[0].body, want)
		}
		require.Len(t, traces.received(), 1)
		assert.Contains(t, traces.received()[0].body, "export bucket missing")
	})
}
