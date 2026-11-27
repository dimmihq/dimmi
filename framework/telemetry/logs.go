/*
|--------------------------------------------------------------------------
| Logs To The Error Tracker
|--------------------------------------------------------------------------
|
| With a Sentry DSN, log lines at the configured level and above go to
| the tracker as well as to the process output, through the SDK's own
| logger: it batches them, sends them in the background and drops the
| oldest when the tracker cannot keep up, so logging never waits on the
| network. A line inside an AI run joins that run's trace; one inside an
| OpenTelemetry trace carries that trace's ids; any other gets a trace
| of its own, since the tracker keeps no line without one.
|
*/

package telemetry

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel/trace"
)

/*
LogHandler returns the handler that sends log records to the tracker,
or nil when no DSN is configured. It sends what it is given as it is:
the logger redacts records before they reach it.
*/
func (t *Telemetry) LogHandler(ctx context.Context) slog.Handler {
	if t == nil || t.hub == nil {
		return nil
	}

	ctx = sentry.SetHubOnContext(context.WithoutCancel(ctx), t.hub)

	return &logHandler{logger: sentry.NewLogger(ctx), level: t.logLevel}
}

/*
logHandler is a slog handler over the SDK's logger. Attributes it was
given and groups it was opened with are kept in prefix and attrs.
*/
type logHandler struct {
	logger sentry.Logger
	level  slog.Level
	prefix string
	attrs  []slog.Attr
}

/*
Enabled reports whether records at the level are sent.
*/
func (h *logHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

/*
Handle queues the record for the tracker. It never fails, so a tracker
that is down never turns into another log line.
*/
func (h *logHandler) Handle(ctx context.Context, record slog.Record) error {
	entry := h.entry(record.Level).WithCtx(context.WithValue(ctx, logTraceKey{}, true))

	for _, a := range h.attrs {
		entry = addAttr(entry, "", a)
	}

	record.Attrs(func(a slog.Attr) bool {
		entry = addAttr(entry, h.prefix, a)

		return true
	})

	entry.Emit(record.Message)

	return nil
}

/*
WithAttrs returns a handler that adds the attributes to every record.
*/
func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append([]slog.Attr{}, h.attrs...)
	for _, a := range attrs {
		next.attrs = append(next.attrs, slog.Attr{Key: h.prefix + a.Key, Value: a.Value})
	}

	return &next
}

/*
WithGroup returns a handler that nests later attributes under the name.
*/
func (h *logHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	next := *h
	next.prefix = h.prefix + name + "."

	return &next
}

func (h *logHandler) entry(level slog.Level) sentry.LogEntry {
	switch {
	case level < slog.LevelInfo:
		return h.logger.Debug()
	case level < slog.LevelWarn:
		return h.logger.Info()
	case level < slog.LevelError:
		return h.logger.Warn()
	default:
		return h.logger.Error()
	}
}

/*
addAttr adds one attribute under its dotted key; a group adds each of
its attributes.
*/
func addAttr(entry sentry.LogEntry, prefix string, a slog.Attr) sentry.LogEntry {
	value := a.Value.Resolve()
	key := prefix + a.Key

	switch value.Kind() {
	case slog.KindGroup:
		inner := key + "."
		if a.Key == "" {
			inner = prefix
		}
		for _, g := range value.Group() {
			entry = addAttr(entry, inner, g)
		}

		return entry
	case slog.KindString:
		return entry.String(key, value.String())
	case slog.KindInt64:
		return entry.Int64(key, value.Int64())
	case slog.KindUint64:
		return entry.String(key, fmt.Sprint(value.Uint64()))
	case slog.KindFloat64:
		return entry.Float64(key, value.Float64())
	case slog.KindBool:
		return entry.Bool(key, value.Bool())
	case slog.KindDuration:
		return entry.String(key, value.Duration().String())
	case slog.KindTime:
		return entry.String(key, value.Time().Format(time.RFC3339Nano))
	case slog.KindAny, slog.KindLogValuer:
		return entry.String(key, fmt.Sprint(value.Any()))
	}

	return entry
}

/*
logTraceKey marks the context of a log line on its way to the tracker,
so traceOf gives trace ids to log lines alone and error reports keep
the trace the SDK finds for them.
*/
type logTraceKey struct{}

/*
traceOf returns the trace a log line belongs to: none of its own inside
an AI run, whose span the SDK reads; the OpenTelemetry trace the
context holds; else a new trace for the line.
*/
func traceOf(ctx context.Context) (sentry.TraceID, sentry.SpanID, bool) {
	if ctx.Value(logTraceKey{}) == nil || sentry.SpanFromContext(ctx) != nil {
		return sentry.TraceID{}, sentry.SpanID{}, false
	}

	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		return sentry.TraceID(span.TraceID()), sentry.SpanID(span.SpanID()), true
	}

	var id sentry.TraceID
	_, _ = rand.Read(id[:])

	return id, sentry.SpanID{}, true
}
