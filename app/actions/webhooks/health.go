/*
|--------------------------------------------------------------------------
| Endpoint Health
|--------------------------------------------------------------------------
|
| An endpoint that answers nothing but failures is first reported, once,
| after a day, then turned off after three, so its owners can fix it
| before deliveries stop. Time alone never turns one off: it must have
| failed a real attempt within the last day, so an endpoint that simply
| had nothing to receive keeps its place. An endpoint answering 410 is
| turned off at once. Turning off cancels what waits to be sent, and the
| owners and admins hear why.
|
*/

package webhooks

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Reasons the system turns an endpoint off.
*/
const (
	disabledAutoFailure = "auto_failure"
	disabledGone        = "gone"
)

/*
sweepBatch is how many due deliveries one sweep queues.
*/
const sweepBatch = 1000

/*
SweepDue queues every delivery whose next attempt is due.
*/
func (w *Webhooks) SweepDue(ctx context.Context) error {
	due, err := models.New(w.DB).ListDueWebhookDeliveries(ctx, models.ListDueWebhookDeliveriesParams{
		DueAt: w.Clock.Now(), RowLimit: sweepBatch,
	})
	if err != nil {
		return fmt.Errorf("list due webhook deliveries: %w", err)
	}

	for _, d := range due {
		err := w.Queue.Push(ctx, jobs.DeliverWebhookArgs{
			WorkspaceID: d.WorkspaceID, DeliveryID: d.ID, Trigger: triggerAuto,
		})
		if err != nil {
			return fmt.Errorf("queue due webhook delivery: %w", err)
		}
	}

	return nil
}

/*
CheckEndpoints reports and turns off failing endpoints, then forgets
expired secrets.
*/
func (w *Webhooks) CheckEndpoints(ctx context.Context) error {
	err := w.eachWorkspace(ctx, func(ctx context.Context, ws ids.UUID, tx *database.Tx, q *models.Queries) error {
		failing, err := q.ListFailingWebhookEndpoints(ctx, models.ListFailingWebhookEndpointsParams{
			WorkspaceID: ws, Since: new(w.Clock.Now().Add(-w.Config.FailingNotifyAfter)),
		})
		if err != nil {
			return fmt.Errorf("list failing webhook endpoints: %w", err)
		}

		for _, e := range failing {
			if err := w.checkEndpoint(ctx, tx, e); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return w.ExpireSecrets(ctx)
}

/*
checkEndpoint turns off an endpoint failing past the limit that failed
recently, and otherwise reports it once.
*/
func (w *Webhooks) checkEndpoint(ctx context.Context, tx *database.Tx, e models.WebhookEndpoint) error {
	q := models.New(tx)
	now := w.Clock.Now()
	if e.FailingSince != nil && !e.FailingSince.After(now.Add(-w.Config.DisableAfter)) {
		recent, err := q.CountRecentWebhookFailures(ctx, models.CountRecentWebhookFailuresParams{
			WorkspaceID: e.WorkspaceID, EndpointID: e.ID, Since: now.Add(-window),
		})
		if err != nil {
			return fmt.Errorf("count recent webhook failures: %w", err)
		}

		if recent > 0 {
			return w.disable(ctx, tx, e, disabledAutoFailure)
		}
	}

	if e.FailingNotifiedAt != nil {
		return nil
	}

	if err := q.MarkWebhookEndpointFailingNotified(ctx, models.MarkWebhookEndpointFailingNotifiedParams{
		WorkspaceID: e.WorkspaceID, ID: e.ID, Now: &now,
	}); err != nil {
		return fmt.Errorf("mark webhook endpoint notified: %w", err)
	}

	lastError := w.lastError(ctx, q, e)

	return w.mailAlert(ctx, tx, e, func(name, workspace string) any {
		return appmail.WebhookEndpointFailingMail{
			Name: name, Workspace: workspace, Host: Host(e.Url), LastError: lastError,
			SettingsURL: w.Links.WebhookEndpoint(EndpointPublicID(e.ID)),
		}
	})
}

/*
lastError names the error of the endpoint's latest failed delivery.
*/
func (w *Webhooks) lastError(ctx context.Context, q *models.Queries, e models.WebhookEndpoint) string {
	failed := statusRetrying

	rows, err := q.ListWebhookDeliveries(ctx, models.ListWebhookDeliveriesParams{
		WorkspaceID: e.WorkspaceID, EndpointID: e.ID, Status: &failed, RowLimit: 1,
	})
	if err != nil || len(rows) == 0 || rows[0].LastErrorCode == nil {
		return "unknown"
	}

	return *rows[0].LastErrorCode
}

/*
disable turns an endpoint off for the reason, cancels what waits to be
sent and tells its owners and admins.
*/
func (w *Webhooks) disable(
	ctx context.Context, tx *database.Tx, e models.WebhookEndpoint, reason string,
) error {
	q := models.New(tx)
	now := w.Clock.Now()

	off, err := q.DisableWebhookEndpoint(ctx, models.DisableWebhookEndpointParams{
		WorkspaceID: e.WorkspaceID, ID: e.ID, DisabledReason: &reason, Now: &now,
	})
	if err != nil {
		return notFoundAs(err, nil, "disable webhook endpoint")
	}

	if _, err := q.CancelWebhookDeliveries(ctx, models.CancelWebhookDeliveriesParams{
		WorkspaceID: e.WorkspaceID, EndpointID: e.ID, Now: now,
	}); err != nil {
		return fmt.Errorf("cancel webhook deliveries: %w", err)
	}

	if err := w.record(ctx, authz.Membership{}, off, audit.Entry{
		Event: "webhook_endpoint.disabled", Changes: map[string]any{"reason": reason},
	}); err != nil {
		return err
	}

	return w.mailAlert(ctx, tx, off, func(name, workspace string) any {
		return appmail.WebhookEndpointDisabledMail{
			Name: name, Workspace: workspace, Host: Host(off.Url), Reason: reason,
			SettingsURL: w.Links.WebhookEndpoint(EndpointPublicID(off.ID)),
		}
	})
}
