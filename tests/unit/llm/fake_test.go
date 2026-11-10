package llm_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

/*
schemaRequest is a request asking for the verdict schema.
*/
func schemaRequest() contracts.ChatRequest {
	req := request("fake-model")
	req.Schema = verdictSchema()
	return req
}

func TestFakeResponseSchema(t *testing.T) {
	t.Parallel()

	t.Run("answers a scripted reply that fits the schema", func(t *testing.T) {
		t.Parallel()
		llm := fake.New(contracts.LLMProviderOpenAI).Script(fake.Step{Text: `{"label":"ham","reason":"fine"}`})

		res, err := llm.Chat(t.Context(), schemaRequest())

		require.NoError(t, err)
		assert.JSONEq(t, `{"label":"ham","reason":"fine"}`, res.Text)
		assert.True(t, llm.Capabilities("fake-model").JSONSchema)
	})

	mismatches := map[string]string{
		"a missing required key":   `{"label":"ham"}`,
		"an extra key":             `{"label":"ham","reason":"x","score":1}`,
		"a value outside the enum": `{"label":"maybe","reason":"x"}`,
		"a wrong type":             `{"label":"ham","reason":3}`,
		"text that is not json":    "```json\n{}\n```",
		"an empty reply":           ``,
	}
	for name, text := range mismatches {
		t.Run("fails a scripted reply with "+name, func(t *testing.T) {
			t.Parallel()
			llm := fake.New(contracts.LLMProviderOpenAI).Script(fake.Step{Text: text})

			_, err := llm.Chat(t.Context(), schemaRequest())

			require.ErrorIs(t, err, fake.ErrSchemaMismatch)
		})
	}

	t.Run("checks nested arrays, alternatives and numbers", func(t *testing.T) {
		t.Parallel()
		req := request("fake-model")
		req.Schema = &contracts.ResponseSchema{Name: "scores", Schema: json.RawMessage(`{
			"type":"object","additionalProperties":false,"required":["items","note"],
			"properties":{
				"items":{"type":"array","items":{"type":"object","additionalProperties":false,
					"required":["n"],"properties":{"n":{"type":"number"}}}},
				"note":{"anyOf":[{"type":"string"},{"type":"null"}]}
			}}`)}
		good := fake.New(contracts.LLMProviderOpenAI).
			Script(fake.Step{Text: `{"items":[{"n":1},{"n":2.5}],"note":null}`})
		bad := fake.New(contracts.LLMProviderOpenAI).Script(fake.Step{Text: `{"items":[{"n":"one"}],"note":true}`})

		_, err := good.Chat(t.Context(), req)
		require.NoError(t, err)
		_, err = bad.Chat(t.Context(), req)
		require.ErrorIs(t, err, fake.ErrSchemaMismatch)
	})

	t.Run("checks the joined deltas of a stream", func(t *testing.T) {
		t.Parallel()
		good := fake.New(contracts.LLMProviderOpenAI).
			Script(fake.Step{Deltas: []string{`{"label":"spam",`, `"reason":"ad"}`}})
		bad := fake.New(contracts.LLMProviderOpenAI).Script(fake.Step{Deltas: []string{`{"label":`, `"spam"}`}})

		stream, err := good.Stream(t.Context(), schemaRequest())
		require.NoError(t, err)
		assert.Len(t, drain(t, stream), 4)
		_, err = bad.Stream(t.Context(), schemaRequest())
		require.ErrorIs(t, err, fake.ErrSchemaMismatch)
	})

	t.Run("refuses a schema when its capabilities lack json schema", func(t *testing.T) {
		t.Parallel()
		llm := fake.New(contracts.LLMProviderOpenAI).
			CapabilitiesOf(contracts.LLMCapabilities{Tools: true}).
			Script(fake.Step{Text: `{"label":"ham","reason":"fine"}`})

		_, chatErr := llm.Chat(t.Context(), schemaRequest())
		_, streamErr := llm.Stream(t.Context(), schemaRequest())

		require.ErrorIs(t, chatErr, contracts.ErrSchemaUnsupported)
		require.ErrorIs(t, streamErr, contracts.ErrSchemaUnsupported)
		assert.Empty(t, llm.Sent())
	})

	t.Run("keeps answering plain text without a schema", func(t *testing.T) {
		t.Parallel()
		llm := fake.New(contracts.LLMProviderOpenAI).Script(fake.Step{Text: "hello"})

		res, err := llm.Chat(t.Context(), request("fake-model"))

		require.NoError(t, err)
		assert.Equal(t, "hello", res.Text)
	})
}
