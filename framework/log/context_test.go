package log_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/dimmihq/dimmi/framework/log"
)

func TestContext(t *testing.T) {
	t.Parallel()

	t.Run("adds context fields to every line", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)
		ctx := log.With(t.Context(), log.String("request_id", "r1"))
		tenantCtx := log.With(ctx, log.String("tenant_id", "t1"))

		logger.InfoContext(tenantCtx, "tenant line")
		logger.InfoContext(ctx, "request line")

		written := lines(t, out)
		require.Len(t, written, 2)
		assert.Equal(t, "r1", written[0]["request_id"])
		assert.Equal(t, "t1", written[0]["tenant_id"])
		assert.Equal(t, "r1", written[1]["request_id"])
		assert.NotContains(t, written[1], "tenant_id")
	})

	t.Run("adds the trace and span ids", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)
		span := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: trace.TraceID{1, 2, 3},
			SpanID:  trace.SpanID{4, 5, 6},
		})
		ctx := trace.ContextWithSpanContext(t.Context(), span)

		logger.InfoContext(ctx, "traced")

		line := onlyLine(t, out)
		assert.Equal(t, span.TraceID().String(), line["trace_id"])
		assert.Equal(t, span.SpanID().String(), line["span_id"])
	})

	t.Run("keeps context fields on derived loggers", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)
		ctx := log.With(t.Context(), log.String("actor_id", "a1"))

		derived := logger.With(log.String("component", "billing")).WithGroup("invoice")
		derived.InfoContext(ctx, "invoice sent", log.Int("number", 7))

		line := onlyLine(t, out)
		assert.Equal(t, "billing", line["component"])
		assert.Equal(t, map[string]any{"number": float64(7), "actor_id": "a1"}, line["invoice"])
	})

	t.Run("returns the logger the context carries", func(t *testing.T) {
		t.Parallel()
		logger, _ := newJSON(t)

		ctx := log.NewContext(t.Context(), logger)

		assert.Same(t, logger, log.From(ctx))
	})

	t.Run("falls back to a working logger", func(t *testing.T) {
		t.Parallel()

		logger := log.From(t.Context())

		require.NotNil(t, logger)
		assert.NotNil(t, logger.Handler())
	})
}
