/*
|--------------------------------------------------------------------------
| Error Reporting
|--------------------------------------------------------------------------
|
| Some failures need a person to look at them, not just a log line.
| Report records the error on the current span and, when a Sentry DSN is
| configured, sends it to Sentry tagged with the trace id, so the report
| and the trace can be read together. Reports are sent in the background
| and never slow the request that failed.
|
*/

package telemetry

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel/trace"
)

/*
Report records the error on the span in the context and sends it to
Sentry when one is configured. A nil error is ignored.
*/
func (t *Telemetry) Report(ctx context.Context, err error) {
	if err == nil {
		return
	}

	trace.SpanFromContext(ctx).RecordError(err)

	if t.reporter == nil {
		return
	}

	scope := sentry.NewScope()
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		scope.SetTag("trace_id", span.TraceID().String())
	}

	t.reporter.CaptureException(err, &sentry.EventHint{Context: ctx}, scope)
}

func (t *Telemetry) setupReporting(cfg Config) error {
	if cfg.SentryDSN == "" {
		return nil
	}

	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		AttachStacktrace: true,
		// Only AI runs start transactions, so every one of them is kept.
		EnableTracing:    true,
		TracesSampleRate: 1,
	})
	if err != nil {
		return fmt.Errorf("create sentry client: %w", err)
	}

	t.reporter = client
	t.hub = sentry.NewHub(client, sentry.NewScope())
	t.shutdowns = append(t.shutdowns, func(ctx context.Context) error {
		defer client.Close()

		if !client.FlushWithContext(ctx) {
			return fmt.Errorf("flush error reports: %w", context.Cause(ctx))
		}

		return nil
	})

	return nil
}

/*
Flush waits until everything queued for the error tracker is sent, or
the context ends, and reports whether it all went.
*/
func (t *Telemetry) Flush(ctx context.Context) bool {
	if t == nil || t.reporter == nil {
		return true
	}

	return t.reporter.FlushWithContext(ctx)
}
