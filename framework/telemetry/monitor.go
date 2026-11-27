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
	"context"
	"strings"

	"github.com/getsentry/sentry-go"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
AISpan is one timed piece of AI work: an agent run, a model call or a
tool call. Its methods do nothing when monitoring is off.
*/
type AISpan struct {
	span *sentry.Span
}

/*
StartAI begins an AI span with the operation, such as
"gen_ai.invoke_agent" or "gen_ai.chat", and its name. It nests under
the AI span already in the context; without one it starts a new trace.
*/
func (t *Telemetry) StartAI(ctx context.Context, op, name string) (context.Context, AISpan) {
	if t == nil || t.hub == nil {
		return ctx, AISpan{}
	}

	if !sentry.HasHubOnContext(ctx) {
		ctx = sentry.SetHubOnContext(ctx, t.hub)
	}

	span := sentry.StartSpan(ctx, op, sentry.WithDescription(name), sentry.WithTransactionName(name))
	span.SetData("gen_ai.operation.name", operationName(op))

	return span.Context(), AISpan{span: span}
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
End finishes the span and, for the first span of a trace, sends it.
*/
func (s AISpan) End() {
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
