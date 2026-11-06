package llm_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/llmretry"
	"github.com/dimmihq/dimmi/config"
)

/*
scripted fails with the queued errors in turn, then answers; a block
call waits for its context instead.
*/
type scripted struct {
	mu    sync.Mutex
	errs  []error
	calls int
	block bool
}

func (s *scripted) Provider() contracts.LLMProvider { return "scripted" }

func (s *scripted) next(ctx context.Context) error {
	s.mu.Lock()
	s.calls++
	block := s.block
	var err error
	if len(s.errs) > 0 {
		err, s.errs = s.errs[0], s.errs[1:]
	}
	s.mu.Unlock()
	if block {
		return &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: canceled(ctx).Error()}
	}
	return err
}

func (s *scripted) Chat(ctx context.Context, _ contracts.ChatRequest) (*contracts.ChatResponse, error) {
	if err := s.next(ctx); err != nil {
		return nil, err
	}
	return &contracts.ChatResponse{Text: "ok"}, nil
}

func (s *scripted) Stream(ctx context.Context, _ contracts.ChatRequest) (contracts.ChatStream, error) {
	if err := s.next(ctx); err != nil {
		return nil, err
	}
	return &silentStream{wait: func() error { return canceled(ctx) }}, nil
}

func (s *scripted) ListModels(context.Context) ([]contracts.LLMModel, error) { return nil, nil }

func (s *scripted) Capabilities(string) contracts.LLMCapabilities { return contracts.LLMCapabilities{} }

func (s *scripted) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

/*
silentStream never sends an event and fails once its context ends.
*/
type silentStream struct {
	wait func() error
}

func (s *silentStream) Next(context.Context) (contracts.StreamEvent, error) {
	return contracts.StreamEvent{}, &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: s.wait().Error()}
}

func (s *silentStream) Close() error { return nil }

func failure(kind contracts.LLMErrorKind, status int) error {
	return &contracts.LLMError{Kind: kind, HTTPStatus: status, Message: string(kind)}
}

func noWait(int) time.Duration { return 0 }

func TestRetryWrapper(t *testing.T) {
	t.Parallel()

	t.Run("retries throttling and conflicts until an answer", func(t *testing.T) {
		t.Parallel()
		inner := &scripted{errs: []error{
			failure(contracts.LLMErrorRateLimited, 429), failure(contracts.LLMErrorServer, 409),
		}}
		w := llmretry.Wrap(inner, llmretry.Options{Backoff: noWait})

		res, err := w.Chat(t.Context(), contracts.ChatRequest{})

		require.NoError(t, err)
		assert.Equal(t, "ok", res.Text)
		assert.Equal(t, 3, inner.count())
	})

	t.Run("never retries a client error or a final kind", func(t *testing.T) {
		t.Parallel()
		for _, err := range []error{
			failure(contracts.LLMErrorServer, 422), failure(contracts.LLMErrorAuth, 401),
			failure(contracts.LLMErrorQuota, 429),
		} {
			inner := &scripted{errs: []error{err}}
			_, got := llmretry.Wrap(inner, llmretry.Options{Backoff: noWait}).Chat(t.Context(), contracts.ChatRequest{})

			require.Error(t, got)
			assert.Equal(t, 1, inner.count())
		}
	})

	t.Run("gives up when the provider asks for a longer pause than allowed", func(t *testing.T) {
		t.Parallel()
		long := &contracts.LLMError{Kind: contracts.LLMErrorRateLimited, HTTPStatus: 429, RetryAfter: time.Minute}
		inner := &scripted{errs: []error{long}}

		_, err := llmretry.Wrap(inner, llmretry.Options{Backoff: noWait}).Stream(t.Context(), contracts.ChatRequest{})

		assert.Equal(t, contracts.LLMErrorRateLimited, llmError(t, err).Kind)
		assert.Equal(t, 1, inner.count())
	})

	t.Run("hands a lasting overload to the spare", func(t *testing.T) {
		t.Parallel()
		busy := failure(contracts.LLMErrorOverloaded, 529)
		inner := &scripted{errs: []error{busy, busy, busy}}
		spare := &scripted{}

		res, err := llmretry.Wrap(inner, llmretry.Options{Backoff: noWait, Fallback: spare}).
			Chat(t.Context(), contracts.ChatRequest{})

		require.NoError(t, err)
		assert.Equal(t, "ok", res.Text)
		assert.Equal(t, 3, inner.count())
		assert.Equal(t, 1, spare.count())
	})

	t.Run("fails fast while the model's breaker is open", func(t *testing.T) {
		t.Parallel()
		clk := clock.NewFake(time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC))
		breakers := llmretry.NewBreakers(clk, config.AIBreakerConfig{
			Failures: 2, Window: time.Minute, Open: time.Minute,
		})
		busy := failure(contracts.LLMErrorOverloaded, 529)
		inner := &scripted{errs: []error{busy, busy}}
		w := llmretry.Wrap(inner, llmretry.Options{
			Clock: clk, MaxAttempts: 2, Backoff: noWait,
			BreakerFor: func(m string) *llmretry.Breaker { return breakers.For(m) },
		})

		_, first := w.Chat(t.Context(), contracts.ChatRequest{Model: "a"})
		_, second := w.Chat(t.Context(), contracts.ChatRequest{Model: "a"})
		_, other := w.Chat(t.Context(), contracts.ChatRequest{Model: "b"})

		require.Error(t, first)
		assert.Equal(t, "circuit open", llmError(t, second).Message)
		require.NoError(t, other)
		assert.Equal(t, 3, inner.count())
	})

	t.Run("bounds a call by its total timeout", func(t *testing.T) {
		t.Parallel()
		clk := clock.NewFake(time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC))
		inner := &scripted{block: true}
		w := llmretry.Wrap(inner, llmretry.Options{Clock: clk, Backoff: noWait})

		done := make(chan error, 1)
		go func() {
			req := contracts.ChatRequest{Timeout: contracts.StepTimeouts{Total: 5 * time.Second}}
			_, err := w.Chat(t.Context(), req)
			done <- err
		}()
		require.NoError(t, clk.BlockUntilContext(t.Context(), 1))
		clk.Travel(5 * time.Second)

		assert.Equal(t, contracts.LLMErrorTimeout, llmError(t, <-done).Kind)
	})

	t.Run("ends a stream that goes quiet past its first-token bound", func(t *testing.T) {
		t.Parallel()
		clk := clock.NewFake(time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC))
		w := llmretry.Wrap(&scripted{}, llmretry.Options{Clock: clk, Backoff: noWait})

		req := contracts.ChatRequest{Timeout: contracts.StepTimeouts{FirstToken: 3 * time.Second}}
		stream, err := w.Stream(t.Context(), req)
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() {
			_, err := stream.Next(t.Context())
			done <- err
		}()
		clk.Travel(3 * time.Second)

		assert.Equal(t, contracts.LLMErrorTimeout, llmError(t, <-done).Kind)
		require.NoError(t, stream.Close())
	})

	t.Run("maps each step to its configured bounds", func(t *testing.T) {
		t.Parallel()
		cfg := config.AITimeoutsConfig{
			GenerateFirst: time.Second, GenerateIdle: 2 * time.Second,
			GenerateTotal: 3 * time.Second, ClassifyTotal: 4 * time.Second,
		}

		assert.Equal(t, contracts.StepTimeouts{FirstToken: time.Second, Idle: 2 * time.Second, Total: 3 * time.Second},
			llmretry.StepTimeouts(cfg, llmretry.StepGenerate))
		assert.Equal(t, contracts.StepTimeouts{Total: 4 * time.Second},
			llmretry.StepTimeouts(cfg, llmretry.StepClassify))
	})
}

func TestBreaker(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	t.Run("reopens after a failed probe and closes after a good one", func(t *testing.T) {
		t.Parallel()
		clk := clock.NewFake(start)
		b := llmretry.NewBreaker(clk, 1, time.Minute, time.Minute)

		b.Failure()
		require.Error(t, b.Allow())
		clk.Travel(2 * time.Minute)
		require.NoError(t, b.Allow(), "the probe goes through")
		require.Error(t, b.Allow(), "only one probe at a time")
		b.Failure()
		require.Error(t, b.Allow(), "a failed probe reopens it")
		clk.Travel(2 * time.Minute)
		require.NoError(t, b.Allow(), "the next cool-down lets a probe through again")
		b.Success()

		assert.NoError(t, b.Allow())
	})

	t.Run("frees the probe slot when the probe is abandoned", func(t *testing.T) {
		t.Parallel()
		clk := clock.NewFake(start)
		b := llmretry.NewBreaker(clk, 1, time.Minute, time.Minute)
		b.Failure()
		clk.Travel(2 * time.Minute)

		require.NoError(t, b.Allow())
		b.Release()

		assert.NoError(t, b.Allow())
	})

	t.Run("forgets failures older than its window", func(t *testing.T) {
		t.Parallel()
		clk := clock.NewFake(start)
		b := llmretry.NewBreaker(clk, 2, time.Minute, time.Minute)

		b.Failure()
		clk.Travel(2 * time.Minute)
		b.Failure()

		assert.NoError(t, b.Allow())
	})
}
