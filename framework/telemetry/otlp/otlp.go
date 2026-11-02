/*
|--------------------------------------------------------------------------
| OTLP Tracing
|--------------------------------------------------------------------------
|
| Spans go to an OpenTelemetry collector over OTLP HTTP. The exporter
| lives apart from telemetry because its library links a gRPC stack the
| HTTP exporter never uses, which roughly doubles a binary's size, so
| only an application that traces pays for it. It opts in once, where it
| builds its foundation options:
|
|   foundation.Options{Telemetry: []telemetry.Option{otlp.Tracing()}}
|
| and then sets telemetry.tracing to "otlp" with the collector's URL in
| telemetry.otlp_endpoint. An empty endpoint falls back to the standard
| OTEL_EXPORTER_OTLP environment variables.
|
*/

package otlp

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
Driver is the tracing driver name the exporter is registered under.
*/
const Driver = "otlp"

/*
Tracing returns the option that makes "otlp" a tracing driver.
*/
func Tracing() telemetry.Option {
	return telemetry.WithExporter(Driver, exporter)
}

/*
exporter sends spans to the configured endpoint with the configured
headers on every request.
*/
func exporter(ctx context.Context, cfg telemetry.Config) (sdktrace.SpanExporter, error) {
	var opts []otlptracehttp.Option
	if cfg.OTLPEndpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint))
	}

	if len(cfg.OTLPHeaders) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.OTLPHeaders))
	}

	spans, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	return spans, nil
}
