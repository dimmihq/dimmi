/*
|--------------------------------------------------------------------------
| LLM Retry
|--------------------------------------------------------------------------
|
| Drivers make exactly one attempt; staying alive through throttling
| and overload lives here. A call retries only when the failure kind
| says it helps, honors the provider's retry hint, and never retries
| once an answer has started streaming, since a half answer cannot be
| unsent. When retries run out, an optional configured spare answers
| in the failed driver's place.
|
*/

package llmretry

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Options tunes the wrapper.
*/
type Options struct {
	Clock       clock.Clock
	MaxAttempts int
	Fallback    contracts.LLM
	Breaker     *Breaker
}

/*
Wrapper is a contracts.LLM that retries one inner driver.
*/
type Wrapper struct {
	inner contracts.LLM
	opts  Options
}

/*
Wrap returns the inner driver with retries.
*/
func Wrap(inner contracts.LLM, opts Options) *Wrapper {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	return &Wrapper{inner: inner, opts: opts}
}

/*
Provider names the inner driver's provider.
*/
func (w *Wrapper) Provider() contracts.LLMProvider { return w.inner.Provider() }

/*
Chat retries the call per the failure kinds.
*/
func (w *Wrapper) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	var last error

	for attempt := 1; attempt <= w.opts.MaxAttempts; attempt++ {
		if err := w.allow(ctx); err != nil {
			return fallbackOr(w, err, func(d contracts.LLM) (*contracts.ChatResponse, error) {
				return d.Chat(ctx, req)
			})
		}

		res, err := w.inner.Chat(ctx, req)
		if err == nil {
			w.succeed()
			return res, nil
		}

		last = err
		if !w.wait(ctx, err, attempt) {
			break
		}
	}

	return fallbackOr(w, last, func(d contracts.LLM) (*contracts.ChatResponse, error) {
		return d.Chat(ctx, req)
	})
}

/*
Stream retries only until the stream has been opened; once tokens
flow, failures pass through untouched.
*/
func (w *Wrapper) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	var last error

	for attempt := 1; attempt <= w.opts.MaxAttempts; attempt++ {
		if err := w.allow(ctx); err != nil {
			return fallbackOr(w, err, func(d contracts.LLM) (contracts.ChatStream, error) {
				return d.Stream(ctx, req)
			})
		}

		stream, err := w.inner.Stream(ctx, req)
		if err == nil {
			w.succeed()
			return stream, nil
		}

		last = err
		if !w.wait(ctx, err, attempt) {
			break
		}
	}

	return fallbackOr(w, last, func(d contracts.LLM) (contracts.ChatStream, error) {
		return d.Stream(ctx, req)
	})
}

/*
ListModels is not retried.
*/
func (w *Wrapper) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	return w.inner.ListModels(ctx)
}

/*
Probe is not retried.
*/
func (w *Wrapper) Probe(ctx context.Context, model string) (*contracts.ProbeResult, error) {
	return w.inner.Probe(ctx, model)
}

/*
Capabilities is not retried.
*/
func (w *Wrapper) Capabilities(model string) contracts.LLMCapabilities {
	return w.inner.Capabilities(model)
}

/*
allow fails fast while the breaker is open.
*/
func (w *Wrapper) allow(ctx context.Context) error {
	if w.opts.Breaker == nil {
		return nil
	}
	if err := w.opts.Breaker.Allow(); err != nil {
		return err
	}
	return ctx.Err()
}

/*
succeed feeds the breaker.
*/
func (w *Wrapper) succeed() {
	if w.opts.Breaker != nil {
		w.opts.Breaker.Success()
	}
}

/*
wait decides whether to retry: only retryable kinds, waits the
provider's hint or a short backoff, and feeds the breaker.
*/
func (w *Wrapper) wait(ctx context.Context, err error, attempt int) bool {
	llmErr := &contracts.LLMError{}
	ok := errors.As(err, &llmErr)
	if !ok || !llmErr.Retryable() {
		return false
	}
	if w.opts.Breaker != nil {
		w.opts.Breaker.Failure()
	}
	if attempt >= w.opts.MaxAttempts {
		return false
	}

	delay := llmErr.RetryAfter
	if delay <= 0 {
		delay = backoffDelay(attempt)
	}

	select {
	case <-ctx.Done():
		return false
	case <-w.opts.Clock.After(delay):
		return true
	}
}

/*
backoffDelay grows with attempts.
*/
func backoffDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 400 * time.Millisecond
	default:
		return 1200 * time.Millisecond
	}
}

/*
fallbackOr hands the call to the configured spare when the failure
kind allows a fallback; otherwise the original error stands.
*/
func fallbackOr[T any](w *Wrapper, err error, call func(contracts.LLM) (T, error)) (T, error) {
	var zero T

	if w.opts.Fallback == nil {
		return zero, err
	}

	llmErr := &contracts.LLMError{}
	ok := errors.As(err, &llmErr)
	if !ok || !fallbackable(llmErr.Kind) {
		return zero, err
	}

	return call(w.opts.Fallback)
}

/*
fallbackable reports whether a failure may move to a spare.
*/
func fallbackable(kind contracts.LLMErrorKind) bool {
	switch kind {
	case contracts.LLMErrorRateLimited, contracts.LLMErrorOverloaded,
		contracts.LLMErrorServer, contracts.LLMErrorTimeout:
		return true
	case contracts.LLMErrorAuth, contracts.LLMErrorPermission, contracts.LLMErrorQuota,
		contracts.LLMErrorInvalidRequest, contracts.LLMErrorModelNotFound,
		contracts.LLMErrorContentFiltered, contracts.LLMErrorNetwork,
		contracts.LLMErrorBlockedHost, contracts.LLMErrorCanceled:
		return false
	}
	return false
}
