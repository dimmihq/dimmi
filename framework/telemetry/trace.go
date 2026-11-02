/*
|--------------------------------------------------------------------------
| Tracing
|--------------------------------------------------------------------------
|
| A span times one piece of work and nests under the span already in the
| context, so a request's work reads as a tree. Spans are batched and
| sent by the registered exporter in the background; recording one never
| waits on the collector, and with tracing off nothing is sent at all.
|
*/

package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

/*
Attr is one key and value on a span or a measurement.
*/
type Attr = attribute.KeyValue

/*
String returns a string attribute.
*/
func String(key, value string) Attr {
	return attribute.String(key, value)
}

/*
Int returns an integer attribute.
*/
func Int(key string, value int) Attr {
	return attribute.Int(key, value)
}

/*
Bool returns a boolean attribute.
*/
func Bool(key string, value bool) Attr {
	return attribute.Bool(key, value)
}

/*
Span is one timed piece of work. End it exactly once, usually with
defer right after Start.
*/
type Span struct {
	span trace.Span
}

/*
Start begins a span named after the work and returns a context that
carries it, so work started from that context nests beneath it.
*/
func (t *Telemetry) Start(ctx context.Context, name string, attrs ...Attr) (context.Context, Span) {
	ctx, span := t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))

	return ctx, Span{span: span}
}

/*
Set adds attributes to the span.
*/
func (s Span) Set(attrs ...Attr) {
	s.span.SetAttributes(attrs...)
}

/*
Fail marks the span as failed and records the error on it.
*/
func (s Span) Fail(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

/*
End finishes the span.
*/
func (s Span) End() {
	s.span.End()
}

/*
setupTracing records spans in batches sent by the exporter, or keeps
the no-op tracer when tracing is off.
*/
func (t *Telemetry) setupTracing(ctx context.Context, cfg Config, res *resource.Resource, exporter Exporter) error {
	if exporter == nil {
		return nil
	}

	spans, err := exporter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create %s exporter: %w", cfg.Tracing, err)
	}

	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(spans), sdktrace.WithResource(res))
	t.tracers = provider
	t.tracer = provider.Tracer(cfg.ServiceName)
	t.shutdowns = append(t.shutdowns, provider.Shutdown)

	return nil
}
