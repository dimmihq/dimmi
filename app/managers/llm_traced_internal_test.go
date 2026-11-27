package managers

import (
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
		"gen_ai.chat", `"gen_ai.request.model":"claude-haiku"`, `"gen_ai.system":"anthropic"`,
		`"gen_ai.usage.input_tokens":300`, `"gen_ai.usage.input_tokens.cached":200`,
		`"gen_ai.usage.output_tokens":12`, `"gen_ai.usage.total_tokens":312`,
		`"gen_ai.response.text":"Your order ships Monday."`, `\"role\":\"user\"`, `\"content\":\"Where is my order?\"`,
	} {
		assert.Contains(t, sent, want)
	}
}
