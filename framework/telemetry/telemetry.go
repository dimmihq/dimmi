/*
|--------------------------------------------------------------------------
| Telemetry
|--------------------------------------------------------------------------
|
| Telemetry shows what the running system is doing: traces follow one
| request through its work, metrics count and time what happens, and
| error reports collect what went wrong. Each part is switched on by
| configuration alone. A part left off costs almost nothing, and callers
| record the same way whether anything listens or not.
|
| Everything belongs to one Telemetry value rather than the process-wide
| OpenTelemetry globals, so tests and parallel instances never share
| state. Shutdown flushes whatever is still buffered.
|
*/

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

/*
ErrUnknownDriver means the tracing or metrics driver is not one this
package provides.
*/
var ErrUnknownDriver = errors.New("telemetry: unknown driver")

const (
	driverNone       = "none"
	driverPrometheus = "prometheus"
)

/*
Config switches each part of telemetry on or off.

ServiceName and Environment label everything recorded. Tracing is none
or the name of an exporter the application registered with
WithExporter, such as otlp from the telemetry/otlp package, which sends
spans over HTTP to OTLPEndpoint, the full URL that receives traces such
as http://collector:4318/v1/traces, with OTLPHeaders on every request.
Metrics is none or prometheus, served by MetricsHandler. Errors are
reported to Sentry only when SentryDSN is set.
*/
type Config struct {
	ServiceName  string            `default:"app"  koanf:"service_name"`
	Environment  string            `koanf:"environment"`
	Tracing      string            `default:"none" koanf:"tracing"`
	OTLPEndpoint string            `koanf:"otlp_endpoint"`
	OTLPHeaders  map[string]string `koanf:"otlp_headers"`
	Metrics      string            `default:"none" koanf:"metrics"`
	SentryDSN    string            `koanf:"sentry_dsn"`
}

/*
Exporter creates the exporter that sends finished spans to a tracing
backend, from the configuration.
*/
type Exporter func(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error)

/*
Option adds to what telemetry can do.
*/
type Option func(s *settings)

/*
settings holds what the options added: the tracing exporters by name.
*/
type settings struct {
	exporters map[string]Exporter
}

/*
WithExporter makes an exporter available as a tracing driver under the
name. Exporters are opt in, so a binary carries the code of only the
backends it can send to; an application that sends spans over OTLP
passes otlp.Tracing().
*/
func WithExporter(name string, exporter Exporter) Option {
	return func(s *settings) {
		s.exporters[name] = exporter
	}
}

/*
Telemetry records traces, metrics and error reports for one process.
*/
type Telemetry struct {
	tracers   trace.TracerProvider
	meters    metric.MeterProvider
	tracer    trace.Tracer
	meter     metric.Meter
	registry  *prometheus.Registry
	reporter  *sentry.Client
	shutdowns []func(context.Context) error
}

/*
New starts the parts of telemetry the configuration switches on. Call
Shutdown before the process exits so buffered data is not lost.
*/
func New(ctx context.Context, cfg Config, options ...Option) (*Telemetry, error) {
	s := settings{exporters: map[string]Exporter{}}
	for _, option := range options {
		option(&s)
	}

	if err := cfg.validate(s); err != nil {
		return nil, err
	}

	res := resource.NewSchemaless(
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("deployment.environment.name", cfg.Environment),
	)

	t := &Telemetry{tracers: tracenoop.NewTracerProvider(), meters: metricnoop.NewMeterProvider()}
	t.tracer = t.tracers.Tracer(cfg.ServiceName)
	t.meter = t.meters.Meter(cfg.ServiceName)

	if err := t.setupReporting(cfg); err != nil {
		return nil, err
	}

	if err := t.setupMetrics(cfg, res); err != nil {
		return nil, errors.Join(err, t.Shutdown(ctx))
	}

	if err := t.setupTracing(ctx, cfg, res, s.exporters[cfg.Tracing]); err != nil {
		return nil, errors.Join(err, t.Shutdown(ctx))
	}

	return t, nil
}

/*
TracerProvider returns the provider every span is recorded with, for
libraries that trace on their own, such as HTTP and database
instrumentation. With tracing off it records nothing.
*/
func (t *Telemetry) TracerProvider() trace.TracerProvider {
	return t.tracers
}

/*
MeterProvider returns the provider every metric is recorded with, for
libraries that measure on their own. With metrics off it records
nothing.
*/
func (t *Telemetry) MeterProvider() metric.MeterProvider {
	return t.meters
}

/*
Shutdown flushes buffered spans and error reports and stops every part,
giving up when the context ends.
*/
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error

	for _, shutdown := range slices.Backward(t.shutdowns) {
		errs = append(errs, shutdown(ctx))
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("shut down telemetry: %w", err)
	}

	return nil
}

/*
validate checks that the drivers exist; a tracing driver exists only
when an option registered its exporter.
*/
func (c Config) validate(s settings) error {
	if _, ok := s.exporters[c.Tracing]; c.Tracing != driverNone && !ok {
		return fmt.Errorf("tracing driver %q: %w", c.Tracing, ErrUnknownDriver)
	}

	if c.Metrics != driverNone && c.Metrics != driverPrometheus {
		return fmt.Errorf("metrics driver %q: %w", c.Metrics, ErrUnknownDriver)
	}

	return nil
}
