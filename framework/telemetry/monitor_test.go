package telemetry_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
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

	t.Run("shares a plain root between agent runs and marks what is inside one", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		ctx, root := tel.StartTrace(t.Context(), "function", "answer request")
		assert.False(t, telemetry.InAgent(ctx))
		runCtx, run := tel.StartAI(ctx, telemetry.OpAgent, "invoke_agent Ada")
		_, chat := tel.StartAI(runCtx, telemetry.OpChat, "chat m")
		assert.True(t, telemetry.InAgent(runCtx))
		chat.End()
		run.End()
		root.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		requests := tracker.received()
		require.Len(t, requests, 1)
		body := requests[0].body
		assert.Contains(t, body, `"op":"function"`)
		assert.Equal(t, 2, strings.Count(body, `"gen_ai.operation.name"`), "the root is not AI work")
		assert.Equal(t, "simulated_customer", telemetry.Task(telemetry.WithTask(t.Context(), "simulated_customer")))
	})

	t.Run("marks a tool and its run failed when the tool panics, and lets the panic go on", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		assert.PanicsWithValue(t, "boom", func() {
			ctx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
			defer run.End()
			_, tool := tel.StartAI(ctx, telemetry.OpTool, "execute_tool lookup")
			defer tool.End()
			panic("boom")
		})
		require.NoError(t, tel.Shutdown(t.Context()))

		body := tracker.received()[0].body
		assert.Equal(t, 2, strings.Count(body, `"error.type":"panic"`), "the tool and its run")
		assert.Equal(t, 2, strings.Count(body, `"status":"internal_error"`))
	})

	t.Run("marks the tool call in the context failed or answered no", func(t *testing.T) {
		t.Parallel()
		tel, tracker := withTracker(t)

		ctx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
		failedCtx, failed := tel.StartAI(ctx, telemetry.OpTool, "execute_tool lookup")
		telemetry.FailTool(failedCtx, "rate_limited")
		failed.End()
		noCtx, no := tel.StartAI(ctx, telemetry.OpTool, "execute_tool refund")
		telemetry.ToolOutcome(noCtx, "pending_approval")
		no.End()
		telemetry.FailTool(ctx, "ignored")
		run.End()
		require.NoError(t, tel.Shutdown(t.Context()))

		body := tracker.received()[0].body
		assert.Contains(t, body, `"error.type":"rate_limited"`)
		assert.Contains(t, body, `"status":"resource_exhausted"`)
		assert.Contains(t, body, `"tool.outcome":"pending_approval"`)
		assert.NotContains(t, body, "ignored", "only a tool span is marked")
	})

	t.Run("names errors by type, never by message", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "timeout", telemetry.ErrorType(fmt.Errorf("call: %w", context.DeadlineExceeded)))
		assert.Equal(t, "canceled", telemetry.ErrorType(context.Canceled))
		assert.Equal(t, "not_found", telemetry.ErrorType(fault.New(fault.NotFound, "not_found", "ana@example.com")))
		assert.Equal(t, "error", telemetry.ErrorType(errors.New("token sk_live_1 rejected")))
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
