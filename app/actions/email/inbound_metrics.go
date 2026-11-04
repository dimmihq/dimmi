/*
|--------------------------------------------------------------------------
| Inbound Email Metrics
|--------------------------------------------------------------------------
|
| An operator should see mail arriving, being turned away and taking long
| without reading anyone's mail. Every figure is labeled by source and
| outcome only: no workspace, address, sender or subject ever becomes a
| label, so the metrics can leave the install safely.
|
*/

package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
latencyBuckets bound the time from a provider receiving mail to the
message being processed, in seconds: the bar is ten seconds.
*/
func latencyBuckets() []float64 {
	return []float64{0.5, 1, 2, 5, 10, 20, 30, 60, 120, 300}
}

/*
sizeBuckets bound the size of received mail, in bytes.
*/
func sizeBuckets() []float64 {
	return []float64{4 << 10, 16 << 10, 64 << 10, 256 << 10, 1 << 20, 4 << 20, 16 << 20, 32 << 20, 64 << 20}
}

/*
InboundMetrics records what happens to received mail.
*/
type InboundMetrics struct {
	Telemetry *telemetry.Telemetry

	processed  telemetry.Counter
	unroutable telemetry.Counter
	auth       telemetry.Counter
	latency    telemetry.Histogram
	size       telemetry.Histogram
}

/*
NewInboundMetrics creates the instruments.
*/
func NewInboundMetrics(t *telemetry.Telemetry) (*InboundMetrics, error) {
	m := &InboundMetrics{Telemetry: t}

	var errs [5]error

	m.processed, errs[0] = t.Counter("dimmi_email_inbound")
	m.unroutable, errs[1] = t.Counter("dimmi_email_inbound_unroutable")
	m.auth, errs[2] = t.Counter("dimmi_email_auth")
	m.latency, errs[3] = t.Histogram("dimmi_email_inbound_latency_seconds", latencyBuckets()...)
	m.size, errs[4] = t.Histogram("dimmi_email_inbound_bytes", sizeBuckets()...)

	if err := errors.Join(errs[:]...); err != nil {
		return nil, fmt.Errorf("inbound email metrics: %w", err)
	}

	return m, nil
}

/*
Received records a delivery a source handed over: its size, or that no
address of any workspace took it.
*/
func (m *InboundMetrics) Received(ctx context.Context, source string, size int64, routed int) {
	if routed == 0 {
		m.unroutable.Add(ctx, 1, telemetry.String("source", source))

		return
	}

	m.size.Record(ctx, float64(size), telemetry.String("source", source))
}

/*
Processed records the outcome of processing one received email, how long
it took since the provider received it, and how its sender checked out.
*/
func (m *InboundMetrics) Processed(ctx context.Context, row models.InboundEmail) {
	classification := "none"
	if row.Classification != nil {
		classification = *row.Classification
	}

	m.processed.Add(ctx, 1, telemetry.String("source", row.Source),
		telemetry.String("classification", classification), telemetry.String("status", row.Status))

	if row.ProcessedAt != nil {
		seconds := row.ProcessedAt.Sub(row.ReceivedAt).Seconds()
		m.latency.Record(ctx, max(seconds, 0), telemetry.String("source", row.Source))
	}

	var auth struct {
		Result string `json:"result"`
	}
	if json.Unmarshal(row.Auth, &auth) == nil && auth.Result != "" {
		m.auth.Add(ctx, 1, telemetry.String("result", auth.Result))
	}
}

/*
MeteredInboundProcessor processes received email inside a trace span and
records the outcome once processing settled. The outcome is read inside a
workspace transaction, the only place row security lets it be seen;
reading it failing loses only the figure.
*/
type MeteredInboundProcessor struct {
	Processor *ProcessInboundEmail
	DB        *database.DB
	Metrics   *InboundMetrics
}

/*
Handle processes the email and records what came of it.
*/
func (p *MeteredInboundProcessor) Handle(ctx context.Context, workspaceID, inboundEmailID ids.UUID) error {
	ctx, span := p.Metrics.Telemetry.Start(ctx, "email.inbound.process")
	defer span.End()

	err := p.Processor.Handle(ctx, workspaceID, inboundEmailID)
	if err != nil {
		span.Fail(err)
	}

	var row models.InboundEmail

	findErr := within(inWorkspace(ctx, workspaceID), p.DB, func(ctx context.Context, q *models.Queries) error {
		var err error
		row, err = q.FindInboundEmail(ctx, models.FindInboundEmailParams{WorkspaceID: workspaceID, ID: inboundEmailID})

		return err
	})
	if findErr == nil && row.Status != string(enums.InboundEmailStatusProcessing) {
		p.Metrics.Processed(ctx, row)
	}

	return err
}
