/*
|--------------------------------------------------------------------------
| Delivery Log
|--------------------------------------------------------------------------
|
| The log shows every delivery of an endpoint, newest first, with each
| attempt's request and the start of its answer, so a developer sees
| exactly what was sent and what their server said. Any delivery can be
| sent again, the same bytes signed afresh; the attempt runs now, and
| whoever asked may wait for its result. A redelivery never restarts
| the schedule. The endpoint's numbers for the last day come from the
| same attempts.
|
*/

package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeliveryFilter narrows an endpoint's log: by status, event type or
event id, and to deliveries before the cursor's, Limit at a time.
*/
type DeliveryFilter struct {
	Status    *string
	EventType *string
	EventID   *ids.UUID
	BeforeAt  *time.Time
	BeforeID  *ids.UUID
	Limit     int32
}

/*
DeliveryDetail is one delivery with its attempts.
*/
type DeliveryDetail struct {
	Delivery models.WebhookDelivery
	Endpoint models.WebhookEndpoint
	Attempts []models.WebhookDeliveryAttempt
}

/*
Stats are an endpoint's attempts over the last day: how many, how many
were delivered, and the median and 95th percentile duration.
*/
type Stats = models.WebhookEndpointStatsRow

/*
ListDeliveries returns a page of an endpoint's log.
*/
func (w *Webhooks) ListDeliveries(
	ctx context.Context, m authz.Membership, endpoint ids.UUID, f DeliveryFilter,
) ([]models.ListWebhookDeliveriesRow, error) {
	if err := requireRead(m); err != nil {
		return nil, err
	}

	var out []models.ListWebhookDeliveriesRow

	err := w.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		if _, err := findEndpoint(ctx, q, m.Workspace.ID, endpoint); err != nil {
			return err
		}

		var err error
		out, err = q.ListWebhookDeliveries(ctx, models.ListWebhookDeliveriesParams{
			WorkspaceID: m.Workspace.ID, EndpointID: endpoint, Status: f.Status, EventType: f.EventType,
			EventID: f.EventID, BeforeAt: f.BeforeAt, BeforeID: f.BeforeID, RowLimit: f.Limit,
		})

		return wrapf("list webhook deliveries", err)
	})

	return out, err
}

/*
ShowDelivery returns one delivery with its endpoint and attempts.
*/
func (w *Webhooks) ShowDelivery(ctx context.Context, m authz.Membership, id ids.UUID) (DeliveryDetail, error) {
	if err := requireRead(m); err != nil {
		return DeliveryDetail{}, err
	}

	var out DeliveryDetail

	err := w.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		out, err = detail(ctx, q, m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
detail reads a delivery, its endpoint and its attempts.
*/
func detail(ctx context.Context, q *models.Queries, ws, id ids.UUID) (DeliveryDetail, error) {
	d, err := q.FindWebhookDelivery(ctx, models.FindWebhookDeliveryParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return DeliveryDetail{}, notFound(err, "find webhook delivery")
	}

	e, err := findEndpoint(ctx, q, ws, d.EndpointID)
	if err != nil {
		return DeliveryDetail{}, err
	}

	attempts, err := q.ListWebhookDeliveryAttempts(ctx, models.ListWebhookDeliveryAttemptsParams{
		WorkspaceID: ws, DeliveryID: id,
	})

	return DeliveryDetail{Delivery: d, Endpoint: e, Attempts: attempts}, wrapf("list webhook attempts", err)
}

/*
Stats returns an endpoint's numbers for the last day.
*/
func (w *Webhooks) Stats(ctx context.Context, m authz.Membership, endpoint ids.UUID) (Stats, error) {
	if err := requireRead(m); err != nil {
		return Stats{}, err
	}

	var out Stats

	err := w.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		out, err = q.WebhookEndpointStats(ctx, models.WebhookEndpointStatsParams{
			WorkspaceID: m.Workspace.ID, EndpointID: endpoint, Since: w.Clock.Now().Add(-window),
		})

		return wrapf("sum webhook attempts", err)
	})

	return out, err
}

/*
Redeliver queues the delivery to be sent again and returns it as it
was before.
*/
func (w *Webhooks) Redeliver(ctx context.Context, m authz.Membership, id ids.UUID) (DeliveryDetail, error) {
	before, err := w.redeliverable(ctx, m, id)
	if err != nil {
		return before, err
	}

	err = w.Queue.Push(ctx, jobs.DeliverWebhookArgs{
		WorkspaceID: m.Workspace.ID, DeliveryID: id, Trigger: triggerManual,
	})

	return before, wrapf("queue webhook redelivery", err)
}

/*
RedeliverNow sends the delivery again right away and returns it with
the outcome of the attempt.
*/
func (w *Webhooks) RedeliverNow(ctx context.Context, m authz.Membership, id ids.UUID) (DeliveryDetail, error) {
	before, err := w.redeliverable(ctx, m, id)
	if err != nil {
		return before, err
	}

	if _, _, err := w.attempt(ctx, m.Workspace.ID, id, triggerManual); err != nil {
		return before, err
	}

	return w.ShowDelivery(ctx, m, id)
}

/*
redeliverable returns the delivery when the member may send it again.
*/
func (w *Webhooks) redeliverable(ctx context.Context, m authz.Membership, id ids.UUID) (DeliveryDetail, error) {
	if err := requireWrite(m); err != nil {
		return DeliveryDetail{}, err
	}

	return w.ShowDelivery(ctx, m, id)
}

/*
pruneBatch is how many deliveries one delete removes.
*/
const pruneBatch = 5000

/*
Prune deletes the deliveries older than the retention, with their
attempts, in every workspace whose records may go.
*/
func (w *Webhooks) Prune(ctx context.Context) error {
	if w.Config.RetentionDays <= 0 {
		return nil
	}

	before := w.Clock.Now().AddDate(0, 0, -w.Config.RetentionDays)

	return w.eachWorkspace(ctx, func(ctx context.Context, ws ids.UUID, _ *database.Tx, q *models.Queries) error {
		if held, err := w.held(ctx, ws); err != nil || held {
			return err
		}

		for {
			n, err := q.PruneWebhookDeliveries(ctx, models.PruneWebhookDeliveriesParams{
				WorkspaceID: ws, Before: before, RowLimit: pruneBatch,
			})
			if err != nil {
				return fmt.Errorf("prune webhook deliveries: %w", err)
			}

			if n < pruneBatch {
				return nil
			}
		}
	})
}

/*
EraseContact deletes the stored payloads about a contact being erased.
Running it again finds nothing more to delete.
*/
func (w *Webhooks) EraseContact(ctx context.Context, ws, contact ids.UUID) error {
	return w.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		_, err := q.DeleteWebhookContactDeliveries(ctx, models.DeleteWebhookContactDeliveriesParams{
			WorkspaceID: ws, ContactID: &contact,
		})

		return wrapf("erase webhook contact deliveries", err)
	})
}
