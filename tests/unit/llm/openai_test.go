package llm_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/engy"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
	"github.com/dimmihq/dimmi/app/drivers/llm/openaicompat"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/llm"
)

const completionURL = "POST https://api.openai.com/v1/chat/completions"

/*
chunk is one streamed completion chunk with text and a finish reason.
*/
func chunk(text, finish string) string {
	finishJSON := "null"
	if finish != "" {
		finishJSON = `"` + finish + `"`
	}
	return `{"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,` +
		`"delta":{"content":"` + text + `"},"finish_reason":` + finishJSON + `}]}`
}

func TestOpenAIDriver(t *testing.T) {
	t.Parallel()

	t.Run("sends the request shape and counts cached tokens once", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(completionURL, fixtures.Reply("openai_completion.json"))
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		req := request("gpt-6.1-sol")
		req.MaxOutputTokens = 300
		req.Tools = []contracts.ToolSpec{{Name: "lookup", Description: "Finds", Schema: json.RawMessage(`{}`)}}
		req.ToolChoice = contracts.ToolChoiceNone
		res, err := drv.Chat(t.Context(), req)

		require.NoError(t, err)
		assert.Equal(t, "Hello", res.Text)
		assert.Equal(t, contracts.StopReasonEnd, res.StopReason)
		assert.Equal(t, int64(40), res.Usage.InputTokens)
		assert.Equal(t, int64(60), res.Usage.CachedInputTokens)
		assert.Equal(t, contracts.LLMProviderOpenAI, drv.Provider())

		sent := body(t, fake.Sent()[0])
		assert.InDelta(t, 300, sent["max_completion_tokens"], 0)
		assert.NotContains(t, sent, "max_tokens")
		assert.NotContains(t, sent, "response_format")
		assert.Equal(t, "none", sent["tool_choice"])
		assert.Equal(t, "Bearer sk-test", fake.Sent()[0].Header.Get("Authorization"))
	})

	t.Run("turns OpenAI reasoning off with tools and keeps GLM reasoning low", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(completionURL, fixtures.Reply("openai_completion.json"))
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		withTools := request("gpt-6-luna")
		withTools.Effort = contracts.EffortLow
		withTools.Tools = []contracts.ToolSpec{{Name: "lookup", Description: "Finds", Schema: json.RawMessage(`{}`)}}
		plain := request("gpt-6-luna")
		plain.Effort = contracts.EffortLow
		_, err := drv.Chat(t.Context(), withTools)
		require.NoError(t, err)
		_, err = drv.Chat(t.Context(), plain)
		require.NoError(t, err)

		assert.Equal(t, "none", body(t, fake.Sent()[0])["reasoning_effort"])
		assert.Equal(t, "low", body(t, fake.Sent()[1])["reasoning_effort"])

		glm := request("glm-5.3-flash")
		glm.Effort = contracts.EffortLow
		glm.Tools = withTools.Tools
		_, err = drv.Chat(t.Context(), glm)
		require.NoError(t, err)
		assert.Equal(t, "low", body(t, fake.Sent()[2])["reasoning_effort"])
	})

	t.Run("streams text, a whole tool call, usage and the stop reason", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(completionURL, fixtures.Reply("openai_tool.sse"))
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		stream, err := drv.Stream(t.Context(), request("gpt-6.1-sol"))
		require.NoError(t, err)
		events := drain(t, stream)

		assert.Equal(t, []contracts.StreamEventKind{
			contracts.StreamEventTextDelta, contracts.StreamEventTextDelta, contracts.StreamEventToolCallDone,
			contracts.StreamEventUsage, contracts.StreamEventDone,
		}, kinds(events))
		call := find(events, contracts.StreamEventToolCallDone).ToolCall
		assert.Equal(t, "call_1", call.ID)
		assert.Equal(t, "lookup", call.Name)
		assert.JSONEq(t, `{"q":"x"}`, string(call.Arguments))
		assert.Equal(t, int64(40), find(events, contracts.StreamEventUsage).Usage.InputTokens)
		assert.Equal(t, contracts.StopReasonToolUse, find(events, contracts.StreamEventDone).StopReason)
		assert.Equal(t, map[string]any{"include_usage": true}, body(t, fake.Sent()[0])["stream_options"])
	})

	t.Run("ends a truncated stream on max tokens and a filtered one on content filter", func(t *testing.T) {
		t.Parallel()
		for finish, want := range map[string]contracts.StopReason{
			"length": contracts.StopReasonMaxTokens, "content_filter": contracts.StopReasonContentFilter,
		} {
			hc, fake := fakeHTTP(t)
			fake.Stub(completionURL, sse("", chunk("Hi", finish), "", "[DONE]"))
			drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

			stream, err := drv.Stream(t.Context(), request("m"))
			require.NoError(t, err)

			assert.Equal(t, want, find(drain(t, stream), contracts.StreamEventDone).StopReason)
		}
	})

	t.Run("maps request failures to kinds with the retry hint", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.StubSequence(completionURL,
			jsonResponse(fhttp.StatusTooManyRequests, apiError("insufficient_quota", "You exceeded your quota")),
			jsonResponse(fhttp.StatusServiceUnavailable, apiError("server_error", "busy"), "Retry-After", "7"),
			jsonResponse(fhttp.StatusUnauthorized, apiError("invalid_api_key", "bad key")),
		)
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		_, quota := drv.Chat(t.Context(), request("m"))
		_, busy := drv.Stream(t.Context(), request("m"))
		_, auth := drv.Chat(t.Context(), request("m"))

		assert.Equal(t, contracts.LLMErrorQuota, llmError(t, quota).Kind)
		assert.Equal(t, contracts.LLMErrorOverloaded, llmError(t, busy).Kind)
		assert.Equal(t, 7*time.Second, llmError(t, busy).RetryAfter)
		assert.Equal(t, contracts.LLMErrorAuth, llmError(t, auth).Kind)
		assert.Equal(t, "bad key", llmError(t, auth).Message)
	})

	t.Run("maps an error sent inside the stream", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(completionURL, sse("", chunk("Hi", ""), "", apiError("rate_limit_exceeded", "slow down")))
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		stream, err := drv.Stream(t.Context(), request("m"))
		require.NoError(t, err)

		assert.Equal(t, contracts.LLMErrorRateLimited, llmError(t, drainErr(t, stream)).Kind)
	})

	t.Run("lets a compatible server read the legacy field without stream usage", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub("POST https://llm.example.test/v1/chat/completions", sse("", chunk("ok", "stop"), "", "[DONE]"))
		cfg := openaicompat.DefaultConfig()
		cfg.SupportsStreamUsage = false
		drv := openaicompat.New(openaicompat.Options{
			BaseURL: "https://llm.example.test/v1", HTTP: hc.StdClient(), Config: cfg,
		})

		req := request("llama")
		req.MaxOutputTokens = 50
		stream, err := drv.Stream(t.Context(), req)
		require.NoError(t, err)
		drain(t, stream)

		sent := body(t, fake.Sent()[0])
		assert.InDelta(t, 50, sent["max_tokens"], 0)
		assert.NotContains(t, sent, "max_completion_tokens")
		assert.NotContains(t, sent, "response_format")
		assert.NotContains(t, sent, "stream_options")
		assert.Equal(t, contracts.LLMProviderOpenAICompatible, drv.Provider())
		assert.False(t, drv.Capabilities("llama").JSONSchema)
	})

	t.Run("reports a compatible server without a model list as unsupported", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub("GET https://llm.example.test/v1/models", jsonResponse(fhttp.StatusNotFound, apiError("x", "no")))
		drv := openaicompat.New(openaicompat.Options{BaseURL: "https://llm.example.test/v1", HTTP: hc.StdClient()})

		_, err := drv.ListModels(t.Context())

		require.ErrorIs(t, err, contracts.ErrLLMNotSupported)
	})

	t.Run("sends engy calls to its endpoint under its own name", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub("POST https://api.engy.ai/v1/chat/completions", sse("", chunk("ok", "stop"), "", "[DONE]"))
		drv := engy.New(engy.Options{APIKey: "sk-engy", HTTP: hc.StdClient()})

		stream, err := drv.Stream(t.Context(), request("glm-5.3-flash"))
		require.NoError(t, err)
		drain(t, stream)

		assert.Equal(t, contracts.LLMProviderEngy, drv.Provider())
		assert.Equal(t, 327_680, drv.Capabilities("glm-5.3-flash").ContextTokens)
		assert.Equal(t, "Bearer sk-engy", fake.Sent()[0].Header.Get("Authorization"))
	})
}
