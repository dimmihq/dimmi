package managers

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/telemetry"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

func TestTracedLLM(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var bodies strings.Builder
	tracker := testkit.Serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		mu.Lock()
		bodies.Write(body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))

	tel, err := telemetry.New(t.Context(), telemetry.Config{
		ServiceName: "dimmi", Tracing: "none", Metrics: "none",
		SentryDSN: strings.Replace(tracker, "http://", "http://public@", 1) + "/1",
	})
	require.NoError(t, err)

	llm := traced(tel, fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{
		Text:  "Your order ships Monday.",
		Usage: contracts.LLMUsage{InputTokens: 300, CachedInputTokens: 200, OutputTokens: 12},
	}))

	stream, err := llm.Stream(t.Context(), contracts.ChatRequest{
		Model: "claude-haiku",
		Messages: []contracts.ChatMessage{{
			Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "Where is my order?"}},
		}},
	})
	require.NoError(t, err)
	for {
		if _, err := stream.Next(t.Context()); err != nil {
			require.True(t, errors.Is(err, io.EOF))

			break
		}
	}
	require.NoError(t, stream.Close())
	require.NoError(t, tel.Shutdown(t.Context()))

	mu.Lock()
	sent := bodies.String()
	mu.Unlock()
	for _, want := range []string{
		"gen_ai.chat", `"gen_ai.agent.name":"background: model_call"`,
		`"gen_ai.request.model":"claude-haiku"`, `"gen_ai.system":"anthropic"`,
		`"gen_ai.usage.input_tokens":300`, `"gen_ai.usage.input_tokens.cached":200`,
		`"gen_ai.usage.output_tokens":12`, `"gen_ai.usage.total_tokens":312`,
		`"gen_ai.response.text":"Your order ships Monday."`, `\"role\":\"user\"`, `\"content\":\"Where is my order?\"`,
	} {
		assert.Contains(t, sent, want)
	}
}

func TestTracedLLMOutsideAgents(t *testing.T) {
	t.Parallel()

	// Tracked starts a stand-in tracker and returns the connection that
	// reports to it and a function returning each transaction sent.
	tracked := func(t *testing.T) (contracts.LLM, *telemetry.Telemetry, func() []transaction) {
		t.Helper()
		var mu sync.Mutex
		var sent []transaction
		tracker := testkit.Serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			lines := strings.Split(string(body), "\n")
			for i := 1; i+1 < len(lines); i += 2 {
				if !strings.Contains(lines[i], `"type":"transaction"`) {
					continue
				}
				var tx transaction
				assert.NoError(t, json.Unmarshal([]byte(lines[i+1]), &tx))
				mu.Lock()
				sent = append(sent, tx)
				mu.Unlock()
			}
			w.WriteHeader(http.StatusOK)
		}))
		tel, err := telemetry.New(t.Context(), telemetry.Config{
			ServiceName: "dimmi", Tracing: "none", Metrics: "none",
			SentryDSN: strings.Replace(tracker, "http://", "http://public@", 1) + "/1",
		})
		require.NoError(t, err)
		llm := traced(tel, fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: `{"ok":true}`}))
		return llm, tel, func() []transaction {
			require.NoError(t, tel.Shutdown(t.Context()))
			mu.Lock()
			defer mu.Unlock()
			return sent
		}
	}

	t.Run("runs a background call inside a run named after its task", func(t *testing.T) {
		t.Parallel()
		llm, _, sent := tracked(t)

		_, err := llm.Chat(t.Context(), contracts.ChatRequest{
			Model:  "gpt-6-luna",
			Schema: &contracts.ResponseSchema{Name: "ticket_fields", Schema: json.RawMessage(`{"type":"object"}`)},
		})
		require.NoError(t, err)

		txs := sent()
		require.Len(t, txs, 1)
		root := txs[0].Contexts.Trace
		// The tracker counts an agent run for each "invoke_agent" span and
		// names it from gen_ai.agent.name (services/gen_ai.rs).
		assert.Equal(t, "gen_ai.invoke_agent", root.Op)
		assert.Equal(t, "invoke_agent", root.Data["gen_ai.operation.name"])
		assert.Equal(t, "background: ticket_fields", root.Data["gen_ai.agent.name"])
		assert.Equal(t, "background.ticket_fields", root.Data["gen_ai.agent.id"])
		require.Len(t, txs[0].Spans, 1)
		chat := txs[0].Spans[0]
		assert.Equal(t, "gen_ai.chat", chat.Op)
		assert.Equal(t, root.SpanID, chat.ParentSpanID)
		assert.Equal(t, "gpt-6-luna", chat.Data["gen_ai.request.model"])
	})

	t.Run("names the run after the task the context gives", func(t *testing.T) {
		t.Parallel()
		llm, _, sent := tracked(t)

		_, err := llm.Chat(telemetry.WithTask(t.Context(), "simulated_customer"), contracts.ChatRequest{Model: "m"})
		require.NoError(t, err)

		txs := sent()
		require.Len(t, txs, 1)
		assert.Equal(t, "background: simulated_customer", txs[0].Contexts.Trace.Data["gen_ai.agent.name"])
	})

	t.Run("adds no run of its own inside an agent run", func(t *testing.T) {
		t.Parallel()
		llm, tel, sent := tracked(t)

		ctx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
		run.Set("gen_ai.agent.name", "Ada")
		_, err := llm.Chat(ctx, contracts.ChatRequest{Model: "m"})
		require.NoError(t, err)
		run.End()

		txs := sent()
		require.Len(t, txs, 1)
		assert.Equal(t, "Ada", txs[0].Contexts.Trace.Data["gen_ai.agent.name"])
		require.Len(t, txs[0].Spans, 1)
		assert.Equal(t, "gen_ai.chat", txs[0].Spans[0].Op)
	})

	t.Run("marks a failed background call's run failed", func(t *testing.T) {
		t.Parallel()
		_, tel, sent := tracked(t)
		llm := traced(tel, fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Err: errors.New("down")}))

		_, err := llm.Chat(t.Context(), contracts.ChatRequest{Model: "m"})
		require.Error(t, err)

		txs := sent()
		require.Len(t, txs, 1)
		assert.Equal(t, "internal_error", txs[0].Contexts.Trace.Status)
		assert.Equal(t, "internal_error", txs[0].Spans[0].Status)
	})
}

/*
transaction is the part of a sent transaction the tests read.
*/
type transaction struct {
	Contexts struct {
		Trace span `json:"trace"`
	} `json:"contexts"`
	Spans []span `json:"spans"`
}

type span struct {
	SpanID       string         `json:"span_id"`
	ParentSpanID string         `json:"parent_span_id"`
	Op           string         `json:"op"`
	Status       string         `json:"status"`
	Data         map[string]any `json:"data"`
}
