/*
|--------------------------------------------------------------------------
| Context Fields
|--------------------------------------------------------------------------
|
| A request gathers who and what it is about as it runs: the request id,
| the tenant, the actor. Those fields ride on the context, so every line
| written for the request carries them without each call site repeating
| them. When the context holds a trace, its ids are added too, so a line
| can be followed to the trace it belongs to.
|
*/

package log

import (
	"context"
	"log/slog"
	"os"
	"slices"

	"go.opentelemetry.io/otel/trace"
)

type (
	fieldsKey struct{}
	loggerKey struct{}
)

/*
With returns a context that adds the attributes to every line logged
with it, after any fields the context already carries.
*/
func With(ctx context.Context, attrs ...Attr) context.Context {
	return context.WithValue(ctx, fieldsKey{}, slices.Concat(fields(ctx), attrs))
}

/*
NewContext returns a context that carries the logger, for code that
receives a context but no logger.
*/
func NewContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

/*
From returns the logger the context carries. A context without one gets
a JSON logger on stderr, so a line is never silently lost.
*/
func From(ctx context.Context) Logger {
	if logger, ok := ctx.Value(loggerKey{}).(Logger); ok {
		return logger
	}

	opts := &slog.HandlerOptions{ReplaceAttr: redactor(nil)}

	return slog.New(contextHandler{Handler: slog.NewJSONHandler(os.Stderr, opts)})
}

func fields(ctx context.Context) []Attr {
	attrs, _ := ctx.Value(fieldsKey{}).([]Attr)

	return attrs
}

/*
contextHandler adds the context's fields and trace ids to each record
before the wrapped handler formats it.
*/
type contextHandler struct {
	slog.Handler
}

/*
Handle adds the fields and trace ids, then hands the record on.
*/
func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	record.AddAttrs(fields(ctx)...)

	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.TraceID().String()),
			slog.String("span_id", span.SpanID().String()),
		)
	}

	return h.Handler.Handle(ctx, record)
}

/*
WithAttrs keeps the context handler around the narrowed handler.
*/
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

/*
WithGroup keeps the context handler around the grouped handler.
*/
func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{Handler: h.Handler.WithGroup(name)}
}
