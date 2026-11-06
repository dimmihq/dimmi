/*
|--------------------------------------------------------------------------
| LLM Retry
|--------------------------------------------------------------------------
|
| Drivers make exactly one attempt; staying alive through throttling
| and overload lives here. A call retries only when the failure kind
| says it helps and the status is not a client error, waits a jittered
| backoff or the provider's hint, and never retries once an answer has
| started streaming, since a half answer cannot be unsent. A provider
| that asks for a longer pause than the wrapper will wait ends the
| retries at once. When retries run out, an optional configured spare
| answers in the failed driver's place.
|
*/

package llmretry

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/resilience"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Options tunes the wrapper. MaxAttempts bounds the attempts of one
call; MaxRetryAfter is the longest provider hint honored; BreakerFor
returns the breaker guarding one model, or nil for none; Fallback is
the spare that answers once retries run out. Backoff is the wait
before attempt n+1 and defaults to a jittered doubling from half a
second.
*/
type Options struct {
	Clock         clock.Clock
	MaxAttempts   int
	MaxRetryAfter time.Duration
	Fallback      contracts.LLM
	BreakerFor    func(model string) *Breaker
	Backoff       func(n int) time.Duration
}

/*
Wrapper is a contracts.LLM that retries one inner driver.
*/
type Wrapper struct {
	inner contracts.LLM
	opts  Options
}

/*
The defaults the wrapper falls back to when an option is unset.
*/
const (
	defaultMaxAttempts   = 3
	defaultMaxRetryAfter = 20 * time.Second
	baseBackoff          = 500 * time.Millisecond
	maxBackoff           = 8 * time.Second
)

/*
Wrap returns the inner driver with retries, breakers and the step
timeouts each request carries.
*/
func Wrap(inner contracts.LLM, opts Options) *Wrapper {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultMaxAttempts
	}
	if opts.MaxRetryAfter <= 0 {
		opts.MaxRetryAfter = defaultMaxRetryAfter
	}
	if opts.Clock == nil {
		opts.Clock = clock.System()
	}
	if opts.Backoff == nil {
		opts.Backoff = backoff
	}
	return &Wrapper{inner: inner, opts: opts}
}

/*
Unwrap returns the driver the wrapper retries.
*/
func (w *Wrapper) Unwrap() contracts.LLM { return w.inner }

/*
Provider names the inner driver's provider.
*/
func (w *Wrapper) Provider() contracts.LLMProvider { return w.inner.Provider() }

/*
Chat retries the call per the failure kinds, inside the request's
total timeout.
*/
func (w *Wrapper) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	ctx, stop := w.deadline(ctx, req.Timeout.Total)
	defer stop()

	res, err := attempt(ctx, w, req, func(ctx context.Context, d contracts.LLM) (*contracts.ChatResponse, error) {
		return d.Chat(ctx, req)
	})
	return res, timedOut(ctx, err)
}

/*
Stream retries only until the stream has been opened; once tokens
flow, failures pass through untouched. The returned stream enforces
the request's first-token, idle and total timeouts.
*/
func (w *Wrapper) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	ctx, watch := newWatchdog(ctx, w.opts.Clock, req.Timeout)

	stream, err := attempt(ctx, w, req, func(ctx context.Context, d contracts.LLM) (contracts.ChatStream, error) {
		return d.Stream(ctx, req)
	})
	if err != nil {
		err = watch.translate(err)
		watch.stop()
		return nil, err
	}

	return &watchedStream{inner: stream, watch: watch}, nil
}

/*
ListModels is not retried.
*/
func (w *Wrapper) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	return w.inner.ListModels(ctx)
}

/*
Capabilities is not retried.
*/
func (w *Wrapper) Capabilities(model string) contracts.LLMCapabilities {
	return w.inner.Capabilities(model)
}

/*
attempt runs the call until it succeeds, fails for good or runs out of
attempts, then hands a fallbackable failure to the spare.
*/
func attempt[T any](ctx context.Context, w *Wrapper, req contracts.ChatRequest,
	call func(context.Context, contracts.LLM) (T, error),
) (T, error) {
	var zero T
	var last error
	breaker := w.breaker(req.Model)

	for n := 1; n <= w.opts.MaxAttempts; n++ {
		if err := allow(ctx, breaker); err != nil {
			last = err
			break
		}

		res, err := call(ctx, w.inner)
		record(breaker, err)
		if err == nil {
			return res, nil
		}

		last = err
		if !w.wait(ctx, err, n) {
			break
		}
	}

	if w.opts.Fallback == nil || !fallbackable(last) {
		return zero, last
	}
	return call(ctx, w.opts.Fallback)
}

/*
breaker returns the model's breaker, nil when none is configured.
*/
func (w *Wrapper) breaker(model string) *Breaker {
	if w.opts.BreakerFor == nil {
		return nil
	}
	return w.opts.BreakerFor(model)
}

/*
allow fails fast while the breaker is open or the caller gave up.
*/
func allow(ctx context.Context, b *Breaker) error {
	if err := ctx.Err(); err != nil {
		return &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: "request canceled"}
	}
	if b == nil {
		return nil
	}
	return b.Allow()
}

/*
record feeds one outcome to the breaker: a retryable failure counts
against the provider, a cancellation says nothing about it, and any
other answer, a refused request included, proves it is up.
*/
func record(b *Breaker, err error) {
	if b == nil {
		return
	}
	llmErr, isLLM := errors.AsType[*contracts.LLMError](err)
	switch {
	case err == nil:
		b.Success()
	case isLLM && llmErr.Kind == contracts.LLMErrorCanceled:
		b.Release()
	case retryable(err):
		b.Failure()
	default:
		b.Success()
	}
}

/*
wait decides whether to retry and sleeps until the next attempt: only
retryable failures, never past the last attempt, and never when the
provider asks for a longer pause than the wrapper honors.
*/
func (w *Wrapper) wait(ctx context.Context, err error, n int) bool {
	if n >= w.opts.MaxAttempts || !retryable(err) {
		return false
	}

	delay := w.opts.Backoff(n)
	if llmErr, ok := errors.AsType[*contracts.LLMError](err); ok && llmErr.RetryAfter > 0 {
		if llmErr.RetryAfter > w.opts.MaxRetryAfter {
			return false
		}
		delay = llmErr.RetryAfter
	}

	select {
	case <-ctx.Done():
		return false
	case <-w.opts.Clock.After(delay):
		return true
	}
}

/*
retryable reports whether a failure may be tried again: a retryable
kind whose status, when it has one, is not a client error other than
a timeout, a conflict or throttling.
*/
func retryable(err error) bool {
	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	if !ok || !llmErr.Retryable() {
		return false
	}
	status := llmErr.HTTPStatus
	if status < 400 || status >= 500 {
		return true
	}
	return status == 408 || status == 409 || status == 429
}

/*
backoff is the wait after the nth failure: a doubling from half a
second, capped, spread by half either way so callers that failed
together do not retry together.
*/
func backoff(n int) time.Duration {
	var wait time.Duration
	policy := resilience.Policy{Initial: baseBackoff, Max: maxBackoff, Multiplier: 2, Jitter: 0.5, Retries: n}
	for wait = range resilience.Backoff(policy) {
		continue
	}
	return wait
}

/*
fallbackable reports whether a failure may move to a spare.
*/
func fallbackable(err error) bool {
	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	if !ok {
		return false
	}
	switch llmErr.Kind {
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
