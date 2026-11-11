/*
|--------------------------------------------------------------------------
| Webhook Jobs
|--------------------------------------------------------------------------
|
| Webhooks leave the request that caused them. An event an endpoint may
| want is fanned out by a job pushed in the event's own transaction, so
| it exists exactly when the change does; that job stores one delivery
| per matching endpoint and pushes a job to send each. A delivery that
| fails waits in its row for its next time, and a sweep every minute
| pushes the ones that are due, so retries follow the application's
| clock. Every five minutes failing endpoints are checked and expired
| secrets forgotten, and every night the log older than its retention
| goes.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
WebhookWork is what the webhook jobs do.
*/
type WebhookWork interface {
	/*
		FanOut stores and queues the event's deliveries.
	*/
	FanOut(ctx context.Context, args FanOutWebhookEventArgs) error

	/*
		Deliver sends one delivery once.
	*/
	Deliver(ctx context.Context, args DeliverWebhookArgs) error

	/*
		SweepDue queues the deliveries due another attempt.
	*/
	SweepDue(ctx context.Context) error

	/*
		CheckEndpoints warns about and turns off failing endpoints and
		forgets expired secrets.
	*/
	CheckEndpoints(ctx context.Context) error

	/*
		Prune deletes the delivery log past its retention.
	*/
	Prune(ctx context.Context) error
}

/*
FanOutWebhookEvent runs a fan-out.
*/
type FanOutWebhookEvent struct{ Work WebhookWork }

/*
Handle fans the event out.
*/
func (w *FanOutWebhookEvent) Handle(ctx context.Context, job *queue.Job[FanOutWebhookEventArgs]) error {
	return w.Work.FanOut(ctx, job.Args)
}

/*
DeliverWebhook runs one send.
*/
type DeliverWebhook struct{ Work WebhookWork }

/*
Handle sends the delivery.
*/
func (w *DeliverWebhook) Handle(ctx context.Context, job *queue.Job[DeliverWebhookArgs]) error {
	return w.Work.Deliver(ctx, job.Args)
}

/*
SweepWebhookDeliveries runs the sweep of due retries.
*/
type SweepWebhookDeliveries struct{ Work WebhookWork }

/*
Handle queues the due deliveries.
*/
func (w *SweepWebhookDeliveries) Handle(ctx context.Context, _ *queue.Job[SweepWebhookDeliveriesArgs]) error {
	return w.Work.SweepDue(ctx)
}

/*
CheckWebhookEndpoints runs the endpoint check.
*/
type CheckWebhookEndpoints struct{ Work WebhookWork }

/*
Handle checks the endpoints.
*/
func (w *CheckWebhookEndpoints) Handle(ctx context.Context, _ *queue.Job[CheckWebhookEndpointsArgs]) error {
	return w.Work.CheckEndpoints(ctx)
}

/*
PruneWebhookDeliveries runs the nightly prune.
*/
type PruneWebhookDeliveries struct{ Work WebhookWork }

/*
Handle prunes the delivery log.
*/
func (w *PruneWebhookDeliveries) Handle(ctx context.Context, _ *queue.Job[PruneWebhookDeliveriesArgs]) error {
	return w.Work.Prune(ctx)
}
