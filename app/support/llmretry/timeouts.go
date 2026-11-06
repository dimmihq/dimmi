/*
|--------------------------------------------------------------------------
| Step Timeouts
|--------------------------------------------------------------------------
|
| A provider that stalls must not stall the conversation. Every request
| may carry three bounds: the time to its first streamed piece, the
| silence allowed between pieces, and the whole call. The wrapper
| enforces them on the app's clock and reports a breach as a timeout,
| which the retry rules and the run's step record already understand.
|
*/

package llmretry

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/config"
)

/*
Step names a pipeline step whose provider call has its own bounds.
*/
type Step string

/*
The steps with configured bounds.
*/
const (
	StepClassify   Step = "classify"
	StepGenerate   Step = "generate"
	StepVerify     Step = "verify"
	StepSummarize  Step = "summarize"
	StepValidation Step = "validation"
	StepProbe      Step = "probe"
)

/*
StepTimeouts returns the bounds a step's request carries in its
Timeout field. Only generation streams, so only it has first-token
and idle bounds.
*/
func StepTimeouts(cfg config.AITimeoutsConfig, step Step) contracts.StepTimeouts {
	switch step {
	case StepGenerate:
		return contracts.StepTimeouts{
			FirstToken: cfg.GenerateFirst, Idle: cfg.GenerateIdle, Total: cfg.GenerateTotal,
		}
	case StepClassify:
		return contracts.StepTimeouts{Total: cfg.ClassifyTotal}
	case StepVerify:
		return contracts.StepTimeouts{Total: cfg.VerifyTotal}
	case StepSummarize:
		return contracts.StepTimeouts{Total: cfg.SummarizeTotal}
	case StepValidation:
		return contracts.StepTimeouts{Total: cfg.ValidationTotal}
	case StepProbe:
		return contracts.StepTimeouts{Total: cfg.ProbeTotal}
	}
	return contracts.StepTimeouts{}
}

/*
errStepTimeout is the cause a breached bound cancels the call with.
*/
var errStepTimeout = errors.New("llm step timed out")

/*
deadline bounds a context by d on the wrapper's clock; zero leaves it
unbounded.
*/
func (w *Wrapper) deadline(ctx context.Context, d time.Duration) (context.Context, func()) {
	if d <= 0 {
		return ctx, func() {}
	}
	ctx, cancel := context.WithCancelCause(ctx)
	timer := w.opts.Clock.AfterFunc(d, func() { cancel(errStepTimeout) })
	return ctx, func() {
		timer.Stop()
		cancel(nil)
	}
}

/*
timedOut turns a failure caused by a breached bound into a timeout.
*/
func timedOut(ctx context.Context, err error) error {
	return breached(context.Cause(ctx), err)
}

/*
breached reports err as a timeout when the cause is a breached bound.
*/
func breached(cause, err error) error {
	if err == nil || !errors.Is(cause, errStepTimeout) {
		return err
	}
	return &contracts.LLMError{Kind: contracts.LLMErrorTimeout, Message: "provider call timed out"}
}

/*
watchdog cancels a stream's context when it breaches a bound: the
total from the start, the first-token bound until the first piece,
then the idle bound between pieces.
*/
type watchdog struct {
	mu      sync.Mutex
	cause   func() error
	cancel  context.CancelCauseFunc
	clock   clock.Clock
	bounds  contracts.StepTimeouts
	total   stopper
	gap     stopper
	stopped bool
}

/*
stopper is a timer that can be disarmed.
*/
type stopper interface{ Stop() bool }

/*
newWatchdog starts the total and first-token timers over a context
derived from the caller's, which the stream must be opened with.
*/
func newWatchdog(parent context.Context, clk clock.Clock,
	bounds contracts.StepTimeouts,
) (context.Context, *watchdog) {
	ctx, cancel := context.WithCancelCause(parent)
	w := &watchdog{cause: func() error { return context.Cause(ctx) }, cancel: cancel, clock: clk, bounds: bounds}
	if bounds.Total > 0 {
		w.total = clk.AfterFunc(bounds.Total, w.fire)
	}
	w.arm(bounds.FirstToken)
	return ctx, w
}

/*
fire cancels the stream for a breached bound.
*/
func (w *watchdog) fire() { w.cancel(errStepTimeout) }

/*
arm restarts the gap timer for d; zero leaves gaps unbounded.
*/
func (w *watchdog) arm(d time.Duration) {
	if w.gap != nil {
		w.gap.Stop()
		w.gap = nil
	}
	if d > 0 {
		w.gap = w.clock.AfterFunc(d, w.fire)
	}
}

/*
seen restarts the idle bound after a piece arrived.
*/
func (w *watchdog) seen() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.stopped {
		w.arm(w.bounds.Idle)
	}
}

/*
stop disarms every timer and releases the context.
*/
func (w *watchdog) stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	w.stopped = true
	w.arm(0)
	if w.total != nil {
		w.total.Stop()
	}
	w.cancel(nil)
}

/*
translate turns a failure caused by a breached bound into a timeout.
*/
func (w *watchdog) translate(err error) error {
	return breached(w.cause(), err)
}

/*
watchedStream is a stream read under its watchdog.
*/
type watchedStream struct {
	inner contracts.ChatStream
	watch *watchdog
}

/*
Next reads the next event, reporting a breached bound as a timeout
and disarming the watchdog once the stream ends.
*/
func (s *watchedStream) Next(ctx context.Context) (contracts.StreamEvent, error) {
	ev, err := s.inner.Next(ctx)
	if err != nil {
		err = s.watch.translate(err)
		s.watch.stop()
		return ev, err
	}
	s.watch.seen()
	return ev, nil
}

/*
Close releases the stream and its watchdog.
*/
func (s *watchedStream) Close() error {
	err := s.inner.Close()
	s.watch.stop()
	return err
}
