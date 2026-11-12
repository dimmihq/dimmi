/*
|--------------------------------------------------------------------------
| Outbound Webhook Wiring
|--------------------------------------------------------------------------
|
| Builds the actions that manage a workspace's webhook endpoints and
| deliver events to them, makes them hear every event in the
| transaction that dispatched it, and registers the jobs that fan an
| event out, send each delivery, sweep due retries, check failing
| endpoints and prune the log. Payloads carry objects in the API's own
| shapes.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/webhooks"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootWebhooks builds the webhook actions, listens for events and
registers the webhook jobs. It runs after the audit log and the
entitlements exist.
*/
func bootWebhooks(a *app.App) error {
	w := &webhooks.Webhooks{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Keys: a.Keys, HTTP: a.HTTP, DNS: a.DNS, Mail: a.Mail,
		Links: a.Links, Audit: a.Actions.RecordAuditLog, Entitlements: a.Entitlements, Retention: a.Retention,
		Limiter: a.Limiter, Snapshots: resources.WebhookSnapshots{DB: a.DB}, Config: a.Config.Webhooks,
		Edition: a.Config.App.Edition, AppURL: a.Application.Config.App.URL,
	}

	a.Actions.Webhooks = w
	w.Listen(a.Events)

	return errors.Join(
		queue.Register(a.Queue, &jobs.FanOutWebhookEvent{Work: w}),
		queue.Register(a.Queue, &jobs.DeliverWebhook{Work: w}),
		queue.Register(a.Queue, &jobs.SweepWebhookDeliveries{Work: w}),
		queue.Register(a.Queue, &jobs.CheckWebhookEndpoints{Work: w}),
		queue.Register(a.Queue, &jobs.PruneWebhookDeliveries{Work: w}),
	)
}
