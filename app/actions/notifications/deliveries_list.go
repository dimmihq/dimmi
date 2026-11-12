/*
|--------------------------------------------------------------------------
| Reading the Delivery Log
|--------------------------------------------------------------------------
|
| A member reads their own deliveries, newest first, or those of one bell
| row. Someone who manages members may read another member's, to answer
| "why did they not hear about it?" for them.
|
*/

package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeliveryFilter narrows the log: one member, which is the caller unless
named, one channel, one status, and the page after a cursor.
*/
type DeliveryFilter struct {
	MemberID *ids.UUID
	Channel  string
	Status   string
	BeforeAt *time.Time
	BeforeID *ids.UUID
	Limit    int32
}

/*
Deliveries returns a page of the delivery log.
*/
func (c *Center) Deliveries(ctx context.Context, m authz.Membership, in DeliveryFilter,
) ([]models.NotificationDelivery, error) {
	member, err := deliveryReader(m, in.MemberID)
	if err != nil {
		return nil, err
	}
	var out []models.NotificationDelivery
	err = c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		out, err = models.New(tx).ListNotificationDeliveries(ctx, models.ListNotificationDeliveriesParams{
			WorkspaceID: m.Workspace.ID, MemberID: &member, Channel: optional(in.Channel),
			Status: optional(in.Status), BeforeAt: in.BeforeAt, BeforeID: in.BeforeID, RowLimit: in.Limit,
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list notification deliveries: %w", err)
	}
	return out, nil
}

/*
NotificationDeliveries returns the deliveries of one of the caller's
bell rows.
*/
func (c *Center) NotificationDeliveries(ctx context.Context, m authz.Membership, notification ids.UUID,
) ([]models.NotificationDelivery, error) {
	if err := requireRead(m); err != nil {
		return nil, err
	}
	var out []models.NotificationDelivery
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListDeliveriesOfNotification(ctx, models.ListDeliveriesOfNotificationParams{
			NotificationID: notification, NotifiableID: keyOf(m), WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID,
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list deliveries of notification: %w", err)
	}
	return out, nil
}

/*
deliveryReader checks the caller may read the member's log and returns
whose it is: their own unless they manage members and named another.
*/
func deliveryReader(m authz.Membership, named *ids.UUID) (ids.UUID, error) {
	if err := requireRead(m); err != nil {
		return ids.UUID{}, err
	}
	if named == nil || *named == m.Member.ID {
		return m.Member.ID, nil
	}
	if !m.Can("members:write") {
		return ids.UUID{}, exceptions.ErrForbidden
	}
	return *named, nil
}
