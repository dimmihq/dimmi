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

	t.Run("marks the end of stable history and the last message as cacheable", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, fixtures.Reply("anthropic_message.json"))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		req := request("claude-opus-5-5")
		req.CacheHint = true
		req.Messages = []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "first"}}},
			{Role: contracts.ChatRoleAssistant, Parts: []contracts.ContentPart{{Text: "answer"}}, Cacheable: true},
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "newest"}}},
		}
		_, err := drv.Chat(t.Context(), req)
		require.NoError(t, err)

		messages := list(body(t, fake.Sent()[0])["messages"])
		require.Len(t, messages, 3)
		marked := func(i int) any {
			content := list(object(messages[i])["content"])
			return object(content[len(content)-1])["cache_control"]
		}
		assert.Nil(t, marked(0))
		assert.Equal(t, map[string]any{"type": "ephemeral"}, marked(1))
		assert.Equal(t, map[string]any{"type": "ephemeral"}, marked(2))
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
		assert.Equal(t, "msg_1", find(events, contracts.StreamEventUsage).RequestID, "the provider's id for the call")
		assert.Equal(t, "claude-opus-5-5", find(events, contracts.StreamEventUsage).Model)
		assert.Equal(t, contracts.StopReasonToolUse, find(events, contracts.StreamEventDone).StopReason)
	})

	t.Run("sends passages as citable search results, cut into sentences", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, fixtures.Reply("anthropic_message.json"))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		req := request("claude-haiku-5-5")
		req.Messages = []contracts.ChatMessage{
			{
				Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "How do refunds work?"}},
				Sources: []contracts.ChatSource{{ID: "101", Title: "Plans", Text: "Pro is $12. Business is $24."}},
			},
			{Role: contracts.ChatRoleAssistant, ToolCalls: []contracts.ToolCall{{ID: "toolu_1", Name: "search_knowledge"}}},
			{
				Role: contracts.ChatRoleTool, ToolCallID: "toolu_1", Parts: []contracts.ContentPart{{Text: `{"results":[]}`}},
				Sources: []contracts.ChatSource{{
					ID: "1", Title: "Refunds", URL: "https://help.test/refunds",
					Text: "Annual plans are refundable within 30 days. Monthly plans are not.\n\n```bash\ncurl -X POST x\n```",
				}},
			},
		}
		_, err := drv.Chat(t.Context(), req)
		require.NoError(t, err)

		messages := list(body(t, fake.Sent()[0])["messages"])
		first := list(object(messages[0])["content"])
		require.Len(t, first, 2, "the carried passage, then the customer's turn")
		carried := object(first[0])
		assert.Equal(t, "search_result", carried["type"])
		assert.Equal(t, "kb:101", carried["source"])
		assert.Equal(t, map[string]any{"enabled": true}, carried["citations"])
		assert.Equal(t, "text", object(first[1])["type"])

		result := object(list(object(messages[2])["content"])[0])
		require.Equal(t, "tool_result", result["type"])
		found := object(list(result["content"])[0])
		assert.Equal(t, "search_result", found["type"])
		assert.Equal(t, "kb:1", found["source"])
		assert.Equal(t, "Refunds (https://help.test/refunds)", found["title"])
		var pieces []string
		for _, block := range list(found["content"]) {
			pieces = append(pieces, object(block)["text"].(string))
		}
		assert.Equal(t, []string{
			"Annual plans are refundable within 30 days.", "Monthly plans are not.", "```bash\ncurl -X POST x\n```",
		}, pieces, "each sentence is citable on its own; code stays whole")
	})

	t.Run("writes the cited passage's marker after each cited stretch", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		cited := `{"type":"search_result_location","source":"kb:3","title":"Refunds","cited_text":"x",` +
			`"search_result_index":0,"start_block_index":0,"end_block_index":1}`
		other := `{"type":"search_result_location","source":"elsewhere","title":"y","cited_text":"y",` +
			`"search_result_index":1,"start_block_index":0,"end_block_index":1}`
		fake.Stub(messagesURL, sse(
			"message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant",`+
				`"model":"claude-haiku-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":5,"output_tokens":1}}}`,
			"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Yes."}}`,
			"content_block_stop", `{"type":"content_block_stop","index":0}`,
			"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":"","citations":[]}}`,
			"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"citations_delta","citation":`+cited+`}}`,
			"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"citations_delta","citation":`+other+`}}`,
			"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":" Refunds take 30 days"}}`,
			"content_block_stop", `{"type":"content_block_stop","index":1}`,
			"content_block_start", `{"type":"content_block_start","index":2,"content_block":{"type":"text","text":""}}`,
			"content_block_delta", `{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"."}}`,
			"content_block_stop", `{"type":"content_block_stop","index":2}`,
			"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}`,
			"message_stop", `{"type":"message_stop"}`,
		))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		stream, err := drv.Stream(t.Context(), request("claude-haiku-5-5"))
		require.NoError(t, err)
		var text string
		for _, ev := range drain(t, stream) {
			text += ev.Text
		}
		assert.Equal(t, "Yes. Refunds take 30 days [3].", text, "only numbered passages become markers")
		assert.False(t, drv.Capabilities("claude-haiku-5-5").Citations, "off until a model is measured to cite reliably")
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
