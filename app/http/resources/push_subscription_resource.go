/*
|--------------------------------------------------------------------------
| Push Subscription Resources
|--------------------------------------------------------------------------
|
| A browser push is enabled on, never with its keys, and the key a
| browser subscribes with.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
PushSubscription is one of a user's devices.
*/
type PushSubscription struct {
	ID            string     `json:"id"`
	Object        string     `json:"object"`
	Platform      string     `json:"platform"`
	DeviceLabel   *string    `json:"device_label"`
	Endpoint      string     `json:"endpoint"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

/*
NewPushSubscription shapes a device for the API.
*/
func NewPushSubscription(s models.PushSubscription) PushSubscription {
	return PushSubscription{
		ID: models.PublicID(models.PrefixPushSubscription, s.ID), Object: "push_subscription", Platform: s.Platform,
		DeviceLabel: s.DeviceLabel, Endpoint: s.Endpoint, LastSuccessAt: s.LastSuccessAt, CreatedAt: s.CreatedAt,
	}
}

/*
PushConfig is what a browser subscribes with.
*/
type PushConfig struct {
	VAPIDPublicKey string `json:"vapid_public_key"`
	KeyID          string `json:"key_id"`
}
