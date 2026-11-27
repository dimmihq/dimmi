package telemetry_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/telemetry"
)

func TestMonitoring(t *testing.T) {
	t.Parallel()

	withTracker := func(t *testing.T) (*telemetry.Telemetry, *collector) {
		t.Helper()
		tracker := newCollector(t)
		cfg := off()
		cfg.Environment = "staging"
		cfg.SentryDSN = strings.Replace(tracker.server.URL, "http://", "http://public@", 1) + "/42"
		tel, err := telemetry.New(t.Context(), cfg)
		require.NoError(t, err)

		return tel, tracker
	}

	t.Run("sends an agent run with its model call nested beneath it", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		ctx, run := tel.StartAI(t.Context(), "gen_ai.invoke_agent", "research_agent")
		run.Set("gen_ai.agent.name", "research_agent")
		_, chat := tel.StartAI(ctx, "gen_ai.chat", "chat claude-haiku")
		chat.Set("gen_ai.usage.input_tokens", 120)
		chat.Fail()
		chat.End()
		run.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		requests := tracker.received()
		require.Len(t, requests, 1)
		for _, want := range []string{
			`"type":"transaction"`, "gen_ai.invoke_agent", "gen_ai.chat", "research_agent",
			`"gen_ai.operation.name":"chat"`, `"gen_ai.usage.input_tokens":120`, "internal_error",
		} {
			assert.Contains(t, requests[0].body, want)
		}
	})

	t.Run("checks a job in when it starts and when it fails", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		done := tel.CheckIn(telemetry.Monitor{Slug: "prune-audit-logs", Schedule: "10 3 * * *", Timezone: "UTC"})
		done(errors.New("database gone"))
		require.NoError(t, tel.Shutdown(t.Context()))

		bodies := ""
		for _, r := range tracker.received() {
			bodies += r.body
		}
		for _, want := range []string{
			`"type":"check_in"`, "prune-audit-logs", `"in_progress"`, `"error"`, "10 3 * * *",
		} {
			assert.Contains(t, bodies, want)
		}
	})

	t.Run("does nothing without a tracker", func(t *testing.T) {
		t.Parallel()
		tel, err := telemetry.New(t.Context(), off())
		require.NoError(t, err)

		ctx, span := tel.StartAI(t.Context(), "gen_ai.chat", "chat")
		span.Set("gen_ai.request.model", "m")
		span.End()
		tel.CheckIn(telemetry.Monitor{Slug: "x", Schedule: "* * * * *"})(nil)
		assert.Equal(t, t.Context(), ctx)
	})
}
