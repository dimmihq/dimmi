/*
|--------------------------------------------------------------------------
| Recovered Failures
|--------------------------------------------------------------------------
|
| Some failures are hidden from the user on purpose: a search answers
| from keywords when its vectors cannot be had, a model call moves to
| another host, a step that could not be saved is skipped. The user's
| answer stays as it was, but the failure is still ours, so it is kept
| in plain sight. The span of the step that degraded is marked failed
| with the cause and what took its place, the agent run around it is
| flagged as degraded without being failed, and the tracker gets an
| error report grouped by the cause, never one group per occurrence.
| Only the cause and the fallback leave the process, never the content.
|
*/

package telemetry

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/getsentry/sentry-go"
)

/*
Recovered records a failure the work recovered from: kind names the
cause, such as "embedding_timeout", and fallback what was done in its
place, such as "keywords_only". It does nothing without a tracker.
*/
func Recovered(ctx context.Context, kind, fallback string) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil || hub.Client() == nil {
		return
	}

	span := sentry.SpanFromContext(ctx)
	if span != nil && (span.Op == OpAgent || !strings.HasPrefix(span.Op, "gen_ai.")) {
		// A run or a plain root is not the step that failed: the failure
		// gets a span of its own beneath it.
		child := span.StartChild("recovered", sentry.WithDescription("recovered "+kind))
		defer child.Finish()
		span = child
	}

	if span != nil {
		failWith(span, kind)
		span.SetData("error.recovered", true)
		span.SetData("fallback", fallback)
	}

	if run, ok := ctx.Value(agentKey{}).(*agentRun); ok {
		run.degrade(kind)
	}

	report(hub, span, kind, fallback)
}

/*
agentRun is the agent run a context is in, with the kinds of failure
it recovered from so far; tool calls may run side by side.
*/
type agentRun struct {
	span  *sentry.Span
	mu    sync.Mutex
	kinds []string
}

/*
degrade flags the run as degraded by the kind of failure.
*/
func (r *agentRun) degrade(kind string) {
	if r.span == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !slices.Contains(r.kinds, kind) {
		r.kinds = append(r.kinds, kind)
	}
	r.span.SetData("degraded", true)
	r.span.SetData("degraded.kinds", strings.Join(r.kinds, ","))
}

/*
report sends one error report for the recovered failure, grouped by its
kind and tied to the span's trace.
*/
func report(hub *sentry.Hub, span *sentry.Span, kind, fallback string) {
	hub.WithScope(func(scope *sentry.Scope) {
		if span != nil {
			scope.SetSpan(span)
		}
		scope.SetLevel(sentry.LevelWarning)
		scope.SetFingerprint([]string{"recovered", kind})
		scope.SetTag("error.type", kind)
		scope.SetTag("error.recovered", "true")
		scope.SetTag("fallback", fallback)
		hub.CaptureMessage("recovered failure: " + kind)
	})
}

/*
Retried records that the step the context is in has been tried again,
count times so far, as the span's retry.count: a retry that then
succeeded is visible on the span without being reported as an error.
*/
func Retried(ctx context.Context, count int) {
	if span := sentry.SpanFromContext(ctx); span != nil {
		span.SetData("retry.count", count)
	}
}
