/*
|--------------------------------------------------------------------------
| Delivery Status
|--------------------------------------------------------------------------
|
| How far a message sent to a customer got: waiting to go, handed to
| the channel, delivered, read, or failed. Customers' own messages are
| not delivered anywhere, so theirs is not applicable.
|
*/

package enums

/*
DeliveryStatus is where an outbound message stands.
*/
type DeliveryStatus string

/*
The delivery statuses.
*/
const (
	DeliveryStatusPending       DeliveryStatus = "pending"
	DeliveryStatusSent          DeliveryStatus = "sent"
	DeliveryStatusDelivered     DeliveryStatus = "delivered"
	DeliveryStatusRead          DeliveryStatus = "read"
	DeliveryStatusFailed        DeliveryStatus = "failed"
	DeliveryStatusNotApplicable DeliveryStatus = "not_applicable"
)

/*
DeliveryStatuses returns every status, in the order of the database
check.
*/
func DeliveryStatuses() []DeliveryStatus {
	return []DeliveryStatus{
		DeliveryStatusPending, DeliveryStatusSent, DeliveryStatusDelivered, DeliveryStatusRead, DeliveryStatusFailed,
		DeliveryStatusNotApplicable,
	}
}
