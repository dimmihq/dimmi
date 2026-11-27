/*
|--------------------------------------------------------------------------
| Monitoring
|--------------------------------------------------------------------------
|
| Beside errors, the error tracker watches two kinds of work. AI runs are
| sent as spans named by the OpenTelemetry GenAI conventions, so each
| agent run reads with its model calls, tools and token counts. Jobs on a
| clock check in when they start and finish, so a run that is missed,
| fails or takes too long is noticed. Without a Sentry DSN both do
| nothing.
|
*/

package telemetry

import (
	"cmp"
	"context"
	"errors"
	"strings"

	"github.com/getsentry/sentry-go"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
)

/*
AISpan is one timed piece of AI work: an agent run, a model call or a
tool call, or the plain span a run of several agents shares. Its
methods do nothing when monitoring is off.
*/
type AISpan struct {
	span *sentry.Span
}

/*
The span operations the error tracker reads as AI work, named by the
OpenTelemetry GenAI conventions Sentry follows.
*/
const (
	OpAgent   = "gen_ai.invoke_agent"
	OpChat    = "gen_ai.chat"
	OpTool    = "gen_ai.execute_tool"
	OpHandoff = "gen_ai.handoff"
)

/*
agentKey marks a context inside an agent run; taskKey names the
background task a model call made outside any agent belongs to.
*/
type (
	agentKey struct{}
	taskKey  struct{}
)

/*
StartAI begins an AI span with the operation, such as OpAgent or
OpChat, and its name. It nests under the span already in the context;
without one it starts a new trace.
*/
func (t *Telemetry) StartAI(ctx context.Context, op, name string) (context.Context, AISpan) {
	ctx, span := t.open(ctx, op, name)
	if span.span != nil {
		span.Set("gen_ai.operation.name", operationName(op))
	}
	if op == OpAgent {
		ctx = context.WithValue(ctx, agentKey{}, true)
	}

	return ctx, span
}

/*
StartTrace begins a plain span that is not AI work itself, so the
tracker counts nothing for it: the root several agent runs share when
one agent hands the work to the next.
*/
func (t *Telemetry) StartTrace(ctx context.Context, op, name string) (context.Context, AISpan) {
	return t.open(ctx, op, name)
}

func (t *Telemetry) open(ctx context.Context, op, name string) (context.Context, AISpan) {
	if t == nil || t.hub == nil {
		return ctx, AISpan{}
	}

	if !sentry.HasHubOnContext(ctx) {
		ctx = sentry.SetHubOnContext(ctx, t.hub)
	}

	span := sentry.StartSpan(ctx, op, sentry.WithDescription(name), sentry.WithTransactionName(name))

	return span.Context(), AISpan{span: span}
}

/*
InAgent reports whether the context is inside an agent run.
*/
func InAgent(ctx context.Context) bool {
	in, _ := ctx.Value(agentKey{}).(bool)

	return in
}

/*
WithTask names the background task the model calls made with the
context belong to, such as "simulated_customer".
*/
func WithTask(ctx context.Context, task string) context.Context {
	return context.WithValue(ctx, taskKey{}, task)
}

/*
Task returns the background task the context names, or "".
*/
func Task(ctx context.Context) string {
	task, _ := ctx.Value(taskKey{}).(string)

	return task
}

/*
Set records a value on the span, such as "gen_ai.request.model".
*/
func (s AISpan) Set(key string, value any) {
	if s.span != nil {
		s.span.SetData(key, value)
	}
}

/*
Fail marks the span as failed.
*/
func (s AISpan) Fail() {
	if s.span != nil {
		s.span.Status = sentry.SpanStatusInternalError
	}
}

/*
FailWith marks the span as failed with a short, machine-readable error
type, such as "timeout" or "unknown_tool", recorded as error.type. The
type is never a message: it carries no customer data.
*/
func (s AISpan) FailWith(errorType string) {
	if s.span != nil {
		failWith(s.span, errorType)
	}
}

/*
FailTool marks the tool call the context is in as failed with the error
type, for a tool that reports a failure without returning an error,
such as an action whose request failed.
*/
func FailTool(ctx context.Context, errorType string) {
	if span := sentry.SpanFromContext(ctx); span != nil && span.Op == OpTool {
		failWith(span, errorType)
	}
}

/*
ToolOutcome records how the tool call the context is in ended when it
ran and the answer was no, such as "pending_approval" or "refused": an
outcome the agent works with, not a failure.
*/
func ToolOutcome(ctx context.Context, outcome string) {
	if span := sentry.SpanFromContext(ctx); span != nil && span.Op == OpTool {
		span.SetData("tool.outcome", outcome)
	}
}

/*
ErrorType names an error for error.type: "timeout" or "canceled" for
an ended context, the error's code when it carries one, else "error".
*/
func ErrorType(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}

	return cmp.Or(fault.CodeOf(err), "error")
}

/*
failWith sets the status the error type maps to, which the tracker
counts as failed whatever it is as long as it is not "ok", and the
error type beside it.
*/
func failWith(span *sentry.Span, errorType string) {
	span.Status = statusOf(errorType)
	span.SetData("error.type", errorType)
}

func statusOf(errorType string) sentry.SpanStatus {
	switch errorType {
	case "timeout":
		return sentry.SpanStatusDeadlineExceeded
	case "canceled":
		return sentry.SpanStatusCanceled
	case "invalid_arguments":
		return sentry.SpanStatusInvalidArgument
	case "unknown_tool", "not_found":
		return sentry.SpanStatusNotFound
	case "forbidden":
		return sentry.SpanStatusPermissionDenied
	case "rate_limited":
		return sentry.SpanStatusResourceExhausted
	case "unavailable":
		return sentry.SpanStatusUnavailable
	case "auth_failed":
		return sentry.SpanStatusUnauthenticated
	}

	return sentry.SpanStatusInternalError
}

/*
End finishes the span and, for the first span of a trace, sends it.
Deferred, it also marks the span failed with the error type "panic"
when the work it times panics, and lets the panic go on.
*/
func (s AISpan) End() {
	if p := recover(); p != nil {
		s.FailWith("panic")
		s.finish()
		panic(p)
	}

	s.finish()
}

func (s AISpan) finish() {
	if s.span != nil {
		s.span.Finish()
	}
}

/*
operationName turns an operation such as "gen_ai.chat" into its
operation name, "chat".
*/
func operationName(op string) string {
	return strings.TrimPrefix(op, "gen_ai.")
}

/*
Monitor is a job on a clock: its slug, its five-field cron schedule
and the timezone the schedule is read in.
*/
type Monitor struct {
	Slug     string
	Schedule string
	Timezone string
}

/*
The minutes a run may start late before it counts as missed, and may
run before it counts as timed out.
*/
const (
	checkInMargin = 5
	maxRuntime    = 30
)

/*
CheckIn tells the tracker a run of the monitored job started, creating
the monitor on the first run. Call the returned function with the run's
error, or nil, when it ends.
*/
func (t *Telemetry) CheckIn(m Monitor) func(err error) {
	if t == nil || t.reporter == nil {
		return func(error) {}
	}

	config := &sentry.MonitorConfig{
		Schedule:      sentry.CrontabSchedule(m.Schedule),
		Timezone:      m.Timezone,
		CheckInMargin: checkInMargin,
		MaxRuntime:    maxRuntime,
	}
	started := clock.System().Now()

	id := t.reporter.CaptureCheckIn(&sentry.CheckIn{
		MonitorSlug: m.Slug,
		Status:      sentry.CheckInStatusInProgress,
	}, config, nil)

	return func(err error) {
		status := sentry.CheckInStatusOK
		if err != nil {
			status = sentry.CheckInStatusError
		}

		checkIn := &sentry.CheckIn{MonitorSlug: m.Slug, Status: status, Duration: clock.System().Since(started)}
		if id != nil {
			checkIn.ID = *id
		}

		t.reporter.CaptureCheckIn(checkIn, config, nil)
	}
}
