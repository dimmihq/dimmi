/*
|--------------------------------------------------------------------------
| Metrics
|--------------------------------------------------------------------------
|
| Metrics count and time what the application does, cheaply enough to
| record on every request. Instruments are created once at boot and used
| from any goroutine. Prometheus scrapes them from MetricsHandler, which
| also exposes the Go runtime and process figures. With metrics off,
| instruments still work and simply record nothing.
|
*/

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

/*
Counter counts events. It only goes up.
*/
type Counter struct {
	counter metric.Int64Counter
}

/*
Histogram records the spread of values such as durations or sizes.
*/
type Histogram struct {
	histogram metric.Float64Histogram
}

/*
Gauge records a value that goes up and down, such as a queue depth.
*/
type Gauge struct {
	gauge metric.Float64Gauge
}

/*
Counter returns the counter with the name. Prometheus shows it with a
_total suffix.
*/
func (t *Telemetry) Counter(name string) (Counter, error) {
	counter, err := t.meter.Int64Counter(name)
	if err != nil {
		return Counter{counter: counter}, fmt.Errorf("create counter %s: %w", name, err)
	}

	return Counter{counter: counter}, nil
}

/*
Histogram returns the histogram with the name. Buckets are the upper
bounds of its buckets; without them a general-purpose set is used.
*/
func (t *Telemetry) Histogram(name string, buckets ...float64) (Histogram, error) {
	var opts []metric.Float64HistogramOption
	if len(buckets) > 0 {
		opts = append(opts, metric.WithExplicitBucketBoundaries(buckets...))
	}

	histogram, err := t.meter.Float64Histogram(name, opts...)
	if err != nil {
		return Histogram{histogram: histogram}, fmt.Errorf("create histogram %s: %w", name, err)
	}

	return Histogram{histogram: histogram}, nil
}

/*
Gauge returns the gauge with the name.
*/
func (t *Telemetry) Gauge(name string) (Gauge, error) {
	gauge, err := t.meter.Float64Gauge(name)
	if err != nil {
		return Gauge{gauge: gauge}, fmt.Errorf("create gauge %s: %w", name, err)
	}

	return Gauge{gauge: gauge}, nil
}

/*
Add increases the counter for the attributes.
*/
func (c Counter) Add(ctx context.Context, n int64, attrs ...Attr) {
	c.counter.Add(ctx, n, metric.WithAttributes(attrs...))
}

/*
Record adds one value to the histogram for the attributes.
*/
func (h Histogram) Record(ctx context.Context, value float64, attrs ...Attr) {
	h.histogram.Record(ctx, value, metric.WithAttributes(attrs...))
}

/*
Set sets the gauge for the attributes.
*/
func (g Gauge) Set(ctx context.Context, value float64, attrs ...Attr) {
	g.gauge.Record(ctx, value, metric.WithAttributes(attrs...))
}

/*
MetricsHandler serves every metric in the Prometheus text format. With
metrics off it answers 404, so a scraper notices the mistake.
*/
func (t *Telemetry) MetricsHandler() http.Handler {
	if t.registry == nil {
		return http.NotFoundHandler()
	}

	return promhttp.HandlerFor(t.registry, promhttp.HandlerOpts{})
}

func (t *Telemetry) setupMetrics(cfg Config, res *resource.Resource) error {
	if cfg.Metrics != driverPrometheus {
		return nil
	}

	registry := prometheus.NewRegistry()

	err := errors.Join(
		registry.Register(collectors.NewGoCollector()),
		registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})),
	)
	if err != nil {
		return fmt.Errorf("register runtime metrics: %w", err)
	}

	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry), otelprometheus.WithoutScopeInfo())
	if err != nil {
		return fmt.Errorf("create prometheus exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter), sdkmetric.WithResource(res))
	t.meters = provider
	t.meter = provider.Meter(cfg.ServiceName)
	t.registry = registry
	t.shutdowns = append(t.shutdowns, provider.Shutdown)

	return nil
}
