package llm_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/anthropic"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/llm"
)

const messagesURL = "POST https://api.anthropic.com/v1/messages"

func TestAnthropicDriver(t *testing.T) {
	t.Parallel()

	t.Run("sends cache markers and leaves sampling out for current models", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, fixtures.Reply("anthropic_message.json"))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		temp := 0.2
		current, legacy := request("claude-opus-5-5"), request("claude-haiku-4-5")
		current.Temperature, legacy.Temperature = &temp, &temp
		res, err := drv.Chat(t.Context(), current)
		require.NoError(t, err)
		_, err = drv.Chat(t.Context(), legacy)
		require.NoError(t, err)

		assert.Equal(t, "Hello", res.Text)
		assert.Equal(t, contracts.StopReasonMaxTokens, res.StopReason)
		assert.Equal(t, contracts.LLMUsage{
			InputTokens: 50, CachedInputTokens: 200, CacheWriteTokens: 10, OutputTokens: 7, Reported: true,
		}, res.Usage)

		first, second := body(t, fake.Sent()[0]), body(t, fake.Sent()[1])
		assert.NotContains(t, first, "temperature")
		assert.InDelta(t, 0.2, second["temperature"], 0.0001)
		assert.InDelta(t, 16000, first["max_tokens"], 0)
		system := list(first["system"])
		require.Len(t, system, 1)
		assert.Equal(t, map[string]any{"type": "ephemeral"}, object(system[0])["cache_control"])
		assert.Equal(t, "sk-ant", fake.Sent()[0].Header.Get("X-Api-Key"))
	})

	t.Run("streams text, a whole tool call, cache usage and the stop reason", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, fixtures.Reply("anthropic_tool.sse"))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		stream, err := drv.Stream(t.Context(), request("claude-opus-5-5"))
		require.NoError(t, err)
		events := drain(t, stream)

		assert.Equal(t, []contracts.StreamEventKind{
			contracts.StreamEventTextDelta, contracts.StreamEventToolCallDone,
			contracts.StreamEventUsage, contracts.StreamEventDone,
		}, kinds(events))
		call := find(events, contracts.StreamEventToolCallDone).ToolCall
		assert.Equal(t, "toolu_1", call.ID)
		assert.JSONEq(t, `{"q":"x"}`, string(call.Arguments))
		usage := find(events, contracts.StreamEventUsage).Usage
		assert.Equal(t, int64(200), usage.CachedInputTokens)
		assert.Equal(t, int64(10), usage.CacheWriteTokens)
		assert.Equal(t, int64(30), usage.OutputTokens)
		assert.Equal(t, contracts.StopReasonToolUse, find(events, contracts.StreamEventDone).StopReason)
	})

	t.Run("fails from stream on a refusal at request time", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, jsonResponse(529, apiError("overloaded_error", "Overloaded")))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		_, err := drv.Stream(t.Context(), request("claude-opus-5-5"))

		llmErr := llmError(t, err)
		assert.Equal(t, contracts.LLMErrorOverloaded, llmErr.Kind)
		assert.True(t, llmErr.Retryable())
		assert.Equal(t, "overloaded_error", llmErr.ProviderCode)
		assert.Equal(t, "Overloaded", llmErr.Message)
	})

	t.Run("fails from stream on an error event before any content", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, fixtures.Reply("anthropic_overloaded.sse"))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		_, err := drv.Stream(t.Context(), request("claude-opus-5-5"))

		assert.Equal(t, contracts.LLMErrorOverloaded, llmError(t, err).Kind)
	})

	t.Run("reports a stream cut before its stop event", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, fixtures.Reply("anthropic_cut.sse"))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		stream, err := drv.Stream(t.Context(), request("claude-opus-5-5"))
		require.NoError(t, err)

		assert.Equal(t, contracts.LLMErrorNetwork, llmError(t, drainErr(t, stream)).Kind)
	})

	t.Run("maps a spend limit to quota and keeps no request url", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.StubSequence(messagesURL,
			jsonResponse(fhttp.StatusTooManyRequests,
				apiError("rate_limit_error", "You have reached your specified API usage limits.")),
			jsonResponse(fhttp.StatusTooManyRequests,
				apiError("rate_limit_error", "Number of requests has exceeded your rate limit"), "Retry-After", "3"),
		)
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		_, spend := drv.Chat(t.Context(), request("claude-opus-5-5"))
		_, throttle := drv.Chat(t.Context(), request("claude-opus-5-5"))

		assert.Equal(t, contracts.LLMErrorQuota, llmError(t, spend).Kind)
		assert.NotContains(t, spend.Error(), "api.anthropic.com")
		assert.Equal(t, contracts.LLMErrorRateLimited, llmError(t, throttle).Kind)
		assert.Equal(t, 3*time.Second, llmError(t, throttle).RetryAfter)
	})

	t.Run("answers capabilities per model", func(t *testing.T) {
		t.Parallel()
		drv := anthropic.New(anthropic.Options{})

		assert.Equal(t, 1_000_000, drv.Capabilities("claude-fable-5-1").ContextTokens)
		assert.Equal(t, 128_000, drv.Capabilities("claude-sonnet-5-5").MaxOutputTokens)
		assert.Equal(t, 200_000, drv.Capabilities("claude-haiku-4-5").ContextTokens)
		assert.Equal(t, 200_000, drv.Capabilities("claude-unknown").ContextTokens)
	})
}
