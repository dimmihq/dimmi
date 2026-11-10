package llm_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/anthropic"
	"github.com/dimmihq/dimmi/app/drivers/llm/engy"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
	"github.com/dimmihq/dimmi/app/drivers/llm/openaicompat"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/llm"
)

/*
schemaOf builds a response schema named verdict from raw JSON.
*/
func schemaOf(raw string) *contracts.ResponseSchema {
	return &contracts.ResponseSchema{Name: "verdict", Schema: json.RawMessage(raw)}
}

func TestStrictSchema(t *testing.T) {
	t.Parallel()

	t.Run("accepts a closed schema with every property required", func(t *testing.T) {
		t.Parallel()
		schema, err := openai.StrictSchema(verdictSchema())

		require.NoError(t, err)
		assert.Equal(t, false, schema["additionalProperties"])
	})

	refusals := map[string]*contracts.ResponseSchema{
		"an open object": schemaOf(`{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}`),
		"an optional property": schemaOf(`{"type":"object","additionalProperties":false,` +
			`"properties":{"a":{"type":"string"},"b":{"type":"string"}},"required":["a"]}`),
		"a nested open object": schemaOf(`{"type":"object","additionalProperties":false,"required":["a"],` +
			`"properties":{"a":{"type":"array","items":{"type":"object","properties":{"x":{"type":"string"}}}}}}`),
		"additional properties allowed":  schemaOf(`{"type":"object","additionalProperties":true}`),
		"a schema that is not an object": schemaOf(`["object"]`),
		"a name with spaces":             {Name: "my verdict", Schema: verdictSchema().Schema},
		"no name":                        {Schema: verdictSchema().Schema},
	}
	for name, schema := range refusals {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()
			_, err := openai.StrictSchema(schema)

			llmErr := llmError(t, err)
			assert.Equal(t, contracts.LLMErrorInvalidRequest, llmErr.Kind)
			assert.Equal(t, "invalid_response_schema", llmErr.ProviderCode)
		})
	}
}

func TestOpenAIResponseSchema(t *testing.T) {
	t.Parallel()

	t.Run("sends a strict json schema response format", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(completionURL, fixtures.Reply("openai_completion.json"))
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		req := request("gpt-6.1-terra")
		req.Schema = verdictSchema()
		_, err := drv.Chat(t.Context(), req)
		require.NoError(t, err)

		format := object(body(t, fake.Sent()[0])["response_format"])
		assert.Equal(t, "json_schema", format["type"])
		inner := object(format["json_schema"])
		assert.Equal(t, "verdict", inner["name"])
		assert.Equal(t, true, inner["strict"])
		assert.Equal(t, []any{"label", "reason"}, object(inner["schema"])["required"])
	})

	t.Run("refuses a loose schema before sending anything", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		req := request("gpt-6.1-terra")
		req.Schema = schemaOf(`{"type":"object","properties":{"a":{"type":"string"}}}`)
		_, err := drv.Stream(t.Context(), req)

		assert.Equal(t, contracts.LLMErrorInvalidRequest, llmError(t, err).Kind)
		assert.Empty(t, fake.Sent())
	})

	t.Run("refuses a model without structured outputs", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		drv := openai.New(openai.Options{APIKey: "sk-test", HTTP: hc.StdClient()})

		req := request("gpt-3.5-turbo")
		req.Schema = verdictSchema()
		_, err := drv.Chat(t.Context(), req)

		require.ErrorIs(t, err, contracts.ErrSchemaUnsupported)
		assert.Empty(t, fake.Sent())
		assert.True(t, openai.Capabilities("gpt-6-luna").JSONSchema)
		assert.True(t, openai.Capabilities("gpt-4o-mini").JSONSchema)
		assert.False(t, openai.Capabilities("gpt-4-turbo").JSONSchema)
	})

	t.Run("sends a schema to a compatible server that declares it", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub("POST https://llm.example.test/v1/chat/completions", fixtures.Reply("openai_completion.json"))
		cfg := openaicompat.DefaultConfig()
		cfg.JSONSchema = true
		drv := openaicompat.New(openaicompat.Options{
			BaseURL: "https://llm.example.test/v1", HTTP: hc.StdClient(), Config: cfg,
		})

		req := request("llama")
		req.Schema = verdictSchema()
		_, err := drv.Chat(t.Context(), req)
		require.NoError(t, err)

		format := object(body(t, fake.Sent()[0])["response_format"])
		assert.Equal(t, "json_schema", format["type"])
		assert.Equal(t, true, object(format["json_schema"])["strict"])
	})

	t.Run("refuses a schema on a compatible server or engy that never declared it", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		compat := openaicompat.New(openaicompat.Options{BaseURL: "https://llm.example.test/v1", HTTP: hc.StdClient()})
		hosted := engy.New(engy.Options{APIKey: "ek", HTTP: hc.StdClient()})

		req := request("glm-5.3-flash")
		req.Schema = verdictSchema()
		_, compatErr := compat.Chat(t.Context(), req)
		_, engyErr := hosted.Chat(t.Context(), req)

		require.ErrorIs(t, compatErr, contracts.ErrSchemaUnsupported)
		require.ErrorIs(t, engyErr, contracts.ErrSchemaUnsupported)
		assert.False(t, hosted.Capabilities("glm-5.3-flash").JSONSchema)
		assert.Empty(t, fake.Sent())
	})
}

const anthropicAnswer = `{"id":"msg_2","type":"message","role":"assistant","model":"claude-opus-5-5",` +
	`"content":[{"type":"tool_use","id":"toolu_9","name":"verdict","input":{"label":"spam","reason":"ad"}}],` +
	`"stop_reason":"tool_use","usage":{"input_tokens":5,"output_tokens":3}}`

func TestAnthropicResponseSchema(t *testing.T) {
	t.Parallel()

	t.Run("forces one strict tool and returns its input as the text", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, jsonResponse(fhttp.StatusOK, anthropicAnswer))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		req := request("claude-opus-5-5")
		req.Schema = verdictSchema()
		res, err := drv.Chat(t.Context(), req)
		require.NoError(t, err)

		assert.JSONEq(t, `{"label":"spam","reason":"ad"}`, res.Text)
		assert.Empty(t, res.ToolCalls)
		assert.Equal(t, contracts.StopReasonEnd, res.StopReason)

		sent := body(t, fake.Sent()[0])
		tools := list(sent["tools"])
		require.Len(t, tools, 1)
		tool := object(tools[0])
		assert.Equal(t, "verdict", tool["name"])
		assert.Equal(t, true, tool["strict"])
		schema := object(tool["input_schema"])
		assert.Equal(t, false, schema["additionalProperties"])
		assert.Equal(t, []any{"label", "reason"}, schema["required"])
		choice := object(sent["tool_choice"])
		assert.Equal(t, "tool", choice["type"])
		assert.Equal(t, "verdict", choice["name"])
		assert.True(t, drv.Capabilities("claude-opus-5-5").JSONSchema)
	})

	t.Run("streams the forced tool's input as text and no tool call", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		fake.Stub(messagesURL, fixtures.Reply("anthropic_tool.sse"))
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		req := request("claude-opus-5-5")
		req.Schema = &contracts.ResponseSchema{Name: "lookup", Schema: json.RawMessage(`{"type":"object"}`)}
		stream, err := drv.Stream(t.Context(), req)
		require.NoError(t, err)
		events := drain(t, stream)

		assert.Equal(t, []contracts.StreamEventKind{
			contracts.StreamEventTextDelta, contracts.StreamEventUsage, contracts.StreamEventDone,
		}, kinds(events))
		assert.JSONEq(t, `{"q":"x"}`, events[0].Text)
		assert.Equal(t, contracts.StopReasonEnd, find(events, contracts.StreamEventDone).StopReason)
	})

	t.Run("refuses a schema alongside the request's own tools", func(t *testing.T) {
		t.Parallel()
		hc, fake := fakeHTTP(t)
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		req := request("claude-opus-5-5")
		req.Schema = verdictSchema()
		req.Tools = []contracts.ToolSpec{{Name: "lookup", Schema: json.RawMessage(`{}`)}}
		_, err := drv.Chat(t.Context(), req)

		assert.Equal(t, contracts.LLMErrorInvalidRequest, llmError(t, err).Kind)
		assert.Empty(t, fake.Sent())
	})

	t.Run("refuses a schema that is not a json object", func(t *testing.T) {
		t.Parallel()
		hc, _ := fakeHTTP(t)
		drv := anthropic.New(anthropic.Options{APIKey: "sk-ant", HTTP: hc.StdClient()})

		req := request("claude-opus-5-5")
		req.Schema = schemaOf(`[]`)
		_, err := drv.Stream(t.Context(), req)

		assert.Equal(t, contracts.LLMErrorInvalidRequest, llmError(t, err).Kind)
	})
}
