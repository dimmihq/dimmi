/*
|--------------------------------------------------------------------------
| Notification Delivery Controller
|--------------------------------------------------------------------------
|
| GET /notification-deliveries lists the caller's delivery log, newest
| first; someone who manages members may name another member. GET
| /notifications/{notification}/deliveries lists how one bell row
| reached the caller on each channel.
|
*/

package v1

import (
	"context"
	"strconv"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NotificationDeliveryController serves the delivery log.
*/
type NotificationDeliveryController struct {
	Center *notifications.Center
	IDs    *ids.Registry
}

/*
NotificationDeliveriesOutput is a page of the log.
*/
type NotificationDeliveriesOutput struct {
	Body struct {
		Data []resources.NotificationDelivery `json:"data"`
		Meta struct {
			NextCursor *string `json:"next_cursor"`
		} `json:"meta"`
	}
}

/*
NotificationDeliveryListOutput is every line of one bell row.
*/
type NotificationDeliveryListOutput struct {
	Body struct {
		Data []resources.NotificationDelivery `json:"data"`
	}
}

/*
Index lists the log.
*/
func (c *NotificationDeliveryController) Index(ctx context.Context, in *requests.IndexNotificationDeliveriesRequest,
) (*NotificationDeliveriesOutput, error) {
	limit := in.Limit
	if limit == 0 {
		limit = 25
	}
	filter := notifications.DeliveryFilter{Channel: in.Channel, Status: in.Status, Limit: limit + 1}
	if in.Member != "" {
		member, err := c.IDs.Decode(string(models.PrefixMember), in.Member)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}
		filter.MemberID = &member
	}
	filter.BeforeAt, filter.BeforeID = readCursor(in.Cursor)
	rows, err := c.Center.Deliveries(ctx, membership(ctx), filter)
	if err != nil {
		return nil, err
	}
	out := &NotificationDeliveriesOutput{}
	if len(rows) > int(limit) {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next := strconv.FormatInt(last.CreatedAt.UnixMicro(), 10) + "_" + last.ID.String()
		out.Body.Meta.NextCursor = &next
	}
	out.Body.Data = resources.NewNotificationDeliveries(rows)
	return out, nil
}

/*
ForNotification lists how one bell row reached the caller.
*/
func (c *NotificationDeliveryController) ForNotification(ctx context.Context,
	in *requests.NotificationDeliveriesOfRequest,
) (*NotificationDeliveryListOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixNotification), in.Notification)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	rows, err := c.Center.NotificationDeliveries(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	out := &NotificationDeliveryListOutput{}
	out.Body.Data = resources.NewNotificationDeliveries(rows)
	return out, nil
}
