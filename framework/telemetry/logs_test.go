package telemetry_test

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
trackedLog is one log as Rustrak reads it from a "log" item container,
{"items":[...]} (apps/server/src/models/log.rs LogItem): timestamp,
trace_id, level and body are required, and a log without a trace id is
dropped; attributes are kept as {"key":{"value":..,"type":..}}.
*/
type trackedLog struct {
	Timestamp  any                       `json:"timestamp"`
	TraceID    string                    `json:"trace_id"`
	SpanID     string                    `json:"span_id"`
	Level      string                    `json:"level"`
	Body       string                    `json:"body"`
	Attributes map[string]map[string]any `json:"attributes"`
}

/*
logsIn reads every log item of the envelopes the collector received.
*/
func logsIn(t *testing.T, requests []request) []trackedLog {
	t.Helper()
	var logs []trackedLog
	for _, r := range requests {
		lines := strings.Split(r.body, "\n")
		for i := 1; i+1 < len(lines); i += 2 {
			var item struct {
				Type string `json:"type"`
			}
			require.NoError(t, json.Unmarshal([]byte(lines[i]), &item))
			if item.Type != "log" {
				continue
			}
			// A log item is a container by this content type, as
			// apps/server/src/ingest/parser.rs is_container reads it.
			require.Contains(t, lines[i], `"content_type":"application/vnd.sentry.items.log+json"`)
			var container struct {
				Items []trackedLog `json:"items"`
			}
			require.NoError(t, json.Unmarshal([]byte(lines[i+1]), &container))
			logs = append(logs, container.Items...)
		}
	}
	return logs
}

func TestLogHandler(t *testing.T) {
	t.Parallel()

	withTracker := func(t *testing.T, level string) (*telemetry.Telemetry, *collector) {
		t.Helper()
		tracker := newCollector(t)
		cfg := off()
		cfg.Environment, cfg.Release, cfg.SentryLogLevel = "staging", "v1.2.3", level
		cfg.SentryDSN = strings.Replace(tracker.server.URL, "http://", "http://public@", 1) + "/42"
		tel, err := telemetry.New(t.Context(), cfg)
		require.NoError(t, err)
		return tel, tracker
	}

	t.Run("sends each level from info up as a log item, without debug", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t, "")
		logger := slog.New(tel.LogHandler(t.Context()))

		logger.DebugContext(t.Context(), "cache warmed")
		logger.InfoContext(t.Context(), "user signed in", slog.String("user_id", "u1"), slog.Int("attempts", 2))
		logger.WarnContext(t.Context(), "slow query")
		logger.With(slog.String("queue", "mail")).WithGroup("job").
			ErrorContext(t.Context(), "job failed", slog.String("kind", "send"))
		require.NoError(t, tel.Shutdown(t.Context()))

		logs := logsIn(t, tracker.received())
		require.Len(t, logs, 3)
		levels := map[string]trackedLog{}
		for _, l := range logs {
			levels[l.Level] = l
			assert.Len(t, l.TraceID, 32, "a log without a trace id is dropped")
			assert.Equal(t, "staging", l.Attributes["sentry.environment"]["value"])
			assert.Equal(t, "v1.2.3", l.Attributes["sentry.release"]["value"])
		}
		assert.Equal(t, "user signed in", levels["info"].Body)
		assert.Equal(t, "u1", levels["info"].Attributes["user_id"]["value"])
		assert.InDelta(t, 2, levels["info"].Attributes["attempts"]["value"], 0)
		assert.Equal(t, "slow query", levels["warn"].Body)
		assert.Equal(t, "mail", levels["error"].Attributes["queue"]["value"])
		assert.Equal(t, "send", levels["error"].Attributes["job.kind"]["value"])
		assert.NotEqual(t, levels["info"].TraceID, levels["warn"].TraceID, "a line outside a trace gets its own")
	})

	t.Run("sends debug when the level says so", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t, "debug")
		slog.New(tel.LogHandler(t.Context())).DebugContext(t.Context(), "cache warmed")
		require.NoError(t, tel.Shutdown(t.Context()))

		logs := logsIn(t, tracker.received())
		require.Len(t, logs, 1)
		assert.Equal(t, "debug", logs[0].Level)
	})

	t.Run("keeps the trace the line was written in", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t, "")
		logger := slog.New(tel.LogHandler(t.Context()))

		traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
		spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
		otelCtx := trace.ContextWithSpanContext(t.Context(), trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
		}))
		logger.InfoContext(otelCtx, "in a request")

		runCtx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
		logger.InfoContext(runCtx, "in an agent run")
		run.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		bodies := map[string]trackedLog{}
		for _, l := range logsIn(t, tracker.received()) {
			bodies[l.Body] = l
		}
		assert.Equal(t, "0123456789abcdef0123456789abcdef", bodies["in a request"].TraceID)
		assert.Equal(t, "0123456789abcdef", bodies["in a request"].SpanID)
		var tx struct {
			Contexts struct {
				Trace struct {
					TraceID string `json:"trace_id"`
				} `json:"trace"`
			} `json:"contexts"`
		}
		for _, r := range tracker.received() {
			if lines := strings.Split(r.body, "\n"); len(lines) > 2 && strings.Contains(lines[1], `"transaction"`) {
				require.NoError(t, json.Unmarshal([]byte(lines[2]), &tx))
			}
		}
		assert.Equal(t, tx.Contexts.Trace.TraceID, bodies["in an agent run"].TraceID)
	})

	t.Run("has no handler without a tracker", func(t *testing.T) {
		t.Parallel()
		tel, err := telemetry.New(t.Context(), off())
		require.NoError(t, err)

		assert.Nil(t, tel.LogHandler(t.Context()))
	})

	t.Run("refuses a level it does not know", func(t *testing.T) {
		t.Parallel()
		cfg := off()
		cfg.SentryDSN, cfg.SentryLogLevel = "http://public@127.0.0.1:1/1", "loud"

		_, err := telemetry.New(t.Context(), cfg)
		require.ErrorIs(t, err, telemetry.ErrInvalidLogLevel)
	})
}
