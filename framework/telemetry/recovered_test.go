package telemetry_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
items returns the payloads of every item of the type the collector
received.
*/
func items(t *testing.T, requests []request, kind string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, r := range requests {
		lines := strings.Split(r.body, "\n")
		for i := 1; i+1 < len(lines); i += 2 {
			if !strings.Contains(lines[i], `"type":"`+kind+`"`) {
				continue
			}
			var item map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[i+1]), &item))
			out = append(out, item)
		}
	}
	return out
}

func spansOf(tx map[string]any) []map[string]any {
	out := []map[string]any{traceOf(tx)}
	spans, _ := tx["spans"].([]any)
	for _, s := range spans {
		span, _ := s.(map[string]any)
		out = append(out, span)
	}
	return out
}

func traceOf(item map[string]any) map[string]any {
	contexts, _ := item["contexts"].(map[string]any)
	trace, _ := contexts["trace"].(map[string]any)
	return trace
}

func data(span map[string]any) map[string]any {
	d, _ := span["data"].(map[string]any)
	return d
}

func TestRecovered(t *testing.T) {
	t.Parallel()

	withTracker := func(t *testing.T) (*telemetry.Telemetry, *collector) {
		t.Helper()
		tracker := newCollector(t)
		cfg := off()
		cfg.SentryDSN = strings.Replace(tracker.server.URL, "http://", "http://public@", 1) + "/42"
		tel, err := telemetry.New(t.Context(), cfg)
		require.NoError(t, err)
		return tel, tracker
	}

	t.Run("fails the step, flags its run and reports one issue per kind", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		ctx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
		toolCtx, tool := tel.StartAI(ctx, telemetry.OpTool, "execute_tool search")
		telemetry.Recovered(toolCtx, "embedding_timeout", "keywords_only")
		tool.End()
		run.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		txs := items(t, tracker.received(), "transaction")
		require.Len(t, txs, 1)
		spans := spansOf(txs[0])
		agent, step := spans[0], spans[1]
		// A span is failed when its status is set and is not "ok"
		// (services/span.rs); the run that recovered keeps no status.
		assert.Nil(t, agent["status"], "the run recovered, so it did not fail")
		assert.Equal(t, true, data(agent)["degraded"])
		assert.Equal(t, "embedding_timeout", data(agent)["degraded.kinds"])
		assert.Equal(t, "deadline_exceeded", step["status"])
		assert.Equal(t, "embedding_timeout", data(step)["error.type"])
		assert.Equal(t, true, data(step)["error.recovered"])
		assert.Equal(t, "keywords_only", data(step)["fallback"])

		events := items(t, tracker.received(), "event")
		require.Len(t, events, 1)
		ev := events[0]
		assert.Equal(t, []any{"recovered", "embedding_timeout"}, ev["fingerprint"], "one issue per kind")
		assert.Equal(t, "warning", ev["level"])
		assert.Equal(t, "recovered failure: embedding_timeout", ev["message"])
		trace := traceOf(ev)
		assert.Equal(t, step["trace_id"], trace["trace_id"], "the issue opens the trace")
		assert.Equal(t, step["span_id"], trace["span_id"])
	})

	t.Run("gives a failure inside a run but outside any step a span of its own", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		ctx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
		telemetry.Recovered(ctx, "memory_save_failed", "skipped")
		run.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		spans := spansOf(items(t, tracker.received(), "transaction")[0])
		require.Len(t, spans, 2)
		assert.Nil(t, spans[0]["status"])
		assert.Equal(t, "recovered", spans[1]["op"])
		assert.Equal(t, "internal_error", spans[1]["status"])
		assert.Equal(t, "memory_save_failed", data(spans[1])["error.type"])
	})

	t.Run("counts the retries of a step that then succeeded, without an issue", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		ctx, chat := tel.StartAI(t.Context(), telemetry.OpChat, "chat m")
		telemetry.Retried(ctx, 1)
		telemetry.Retried(ctx, 2)
		chat.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		spans := spansOf(items(t, tracker.received(), "transaction")[0])
		assert.InDelta(t, 2, data(spans[0])["retry.count"], 0)
		assert.Nil(t, spans[0]["status"])
		assert.Empty(t, items(t, tracker.received(), "event"))
	})

	t.Run("does nothing without a tracker", func(t *testing.T) {
		t.Parallel()
		telemetry.Recovered(t.Context(), "embedding_timeout", "keywords_only")
		telemetry.Retried(t.Context(), 1)
	})
}
