/*
|--------------------------------------------------------------------------
| Webhook Delivery Controllers
|--------------------------------------------------------------------------
|
| The delivery log of an endpoint: GET .../deliveries pages through it,
| GET /webhook-deliveries/{delivery} shows one with its attempts, and
| POST .../redeliveries sends it again now.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/actions/webhooks"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WebhookDeliveriesOutput is a page of an endpoint's log.
*/
type WebhookDeliveriesOutput struct {
	Body fhttp.Collection[resources.WebhookDelivery]
}

/*
WebhookDeliveryOutput is one delivery with its attempts.
*/
type WebhookDeliveryOutput struct {
	Body fhttp.Resource[resources.WebhookDeliveryDetail]
}

/*
Deliveries lists a page of an endpoint's log, newest first.
*/
func (c *WebhookController) Deliveries(ctx context.Context, in *requests.WebhookDeliveriesRequest,
) (*WebhookDeliveriesOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	m := membership(ctx)

	f, err := c.deliveryFilter(ctx, in)
	if err != nil {
		return nil, err
	}

	rows, err := c.Webhooks.ListDeliveries(ctx, m, id, f)
	if err != nil {
		return nil, err
	}

	limit := int(f.Limit) - 1
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}

	items := make([]resources.WebhookDelivery, 0, len(rows))
	for _, d := range rows {
		items = append(items, resources.NewWebhookDeliveryRow(d))
	}

	page := pagination.Page[resources.WebhookDelivery]{Items: items, Limit: limit}
	if more && len(rows) > 0 {
		page.NextCursor = models.PublicID(models.PrefixWebhookDelivery, rows[len(rows)-1].ID)
	}

	return &WebhookDeliveriesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
ShowDelivery reads one delivery with its payload and attempts.
*/
func (c *WebhookController) ShowDelivery(ctx context.Context, in *requests.WebhookDeliveryRequest,
) (*WebhookDeliveryOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixWebhookDelivery, in.Delivery)
	if err != nil {
		return nil, err
	}

	return deliveryOutput(c.Webhooks.ShowDelivery(ctx, membership(ctx), id))
}

/*
Redeliver sends a delivery again now and answers it with the attempt.
*/
func (c *WebhookController) Redeliver(ctx context.Context, in *requests.WebhookDeliveryRequest,
) (*WebhookDeliveryOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixWebhookDelivery, in.Delivery)
	if err != nil {
		return nil, err
	}

	return deliveryOutput(c.Webhooks.RedeliverNow(ctx, membership(ctx), id))
}

/*
deliveryFilter reads the log's filters, asking one more row than the
page holds to tell whether another page follows. The cursor is the
last delivery seen.
*/
func (c *WebhookController) deliveryFilter(ctx context.Context, in *requests.WebhookDeliveriesRequest,
) (webhooks.DeliveryFilter, error) {
	limit := in.Limit
	if limit <= 0 || limit > maxWebhookDeliveries {
		limit = defaultWebhookDeliveries
	}

	f := webhooks.DeliveryFilter{Limit: limit + 1}
	if in.Status != "" {
		f.Status = &in.Status
	}

	if in.EventType != "" {
		f.EventType = &in.EventType
	}

	if in.Cursor == "" {
		return f, nil
	}

	before, err := decodeID(c.IDs, models.PrefixWebhookDelivery, in.Cursor)
	if err != nil {
		return f, err
	}

	d, err := c.Webhooks.ShowDelivery(ctx, membership(ctx), before)
	if err != nil {
		return f, err
	}

	f.BeforeAt, f.BeforeID = &d.Delivery.CreatedAt, &d.Delivery.ID

	return f, nil
}

/*
deliveryOutput shapes a delivery or passes its error on.
*/
func deliveryOutput(d webhooks.DeliveryDetail, err error) (*WebhookDeliveryOutput, error) {
	if err != nil {
		return nil, err
	}

	return &WebhookDeliveryOutput{Body: fhttp.NewResource(resources.NewWebhookDeliveryDetail(d))}, nil
}
