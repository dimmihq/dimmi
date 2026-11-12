/*
|--------------------------------------------------------------------------
| Notification Delivery Resources
|--------------------------------------------------------------------------
|
| One line of a teammate's delivery log: which alert, on which channel,
| how it ended and why. Ids are public ones; the delivery id groups the
| channels of one alert.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
NotificationDelivery is one delivery of an alert on one channel.
*/
type NotificationDelivery struct {
	ID         string    `json:"id"`
	Object     string    `json:"object"`
	DeliveryID string    `json:"delivery_id"`
	MemberID   *string   `json:"member_id"`
	Trigger    string    `json:"trigger"`
	Channel    string    `json:"channel"`
	Status     string    `json:"status"`
	Reason     *string   `json:"reason"`
	Detail     *string   `json:"detail"`
	Title      string    `json:"title"`
	Link       string    `json:"link"`
	Attempts   int32     `json:"attempts"`
	OccurredAt time.Time `json:"occurred_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

/*
NewNotificationDeliveries shapes log lines for the API.
*/
func NewNotificationDeliveries(rows []models.NotificationDelivery) []NotificationDelivery {
	out := make([]NotificationDelivery, 0, len(rows))
	for _, d := range rows {
		line := NotificationDelivery{
			ID: models.PublicID(models.PrefixNotificationDelivery, d.ID), Object: "notification_delivery",
			DeliveryID: d.DeliveryID.String(), Trigger: d.Trigger, Channel: d.Channel, Status: d.Status,
			Reason: d.Reason, Detail: d.Detail, Title: d.Title, Link: d.Link, Attempts: d.Attempts,
			OccurredAt: d.OccurredAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
		}
		if d.WorkspaceMemberID != nil {
			member := models.PublicID(models.PrefixMember, *d.WorkspaceMemberID)
			line.MemberID = &member
		}
		out = append(out, line)
	}
	return out
}
