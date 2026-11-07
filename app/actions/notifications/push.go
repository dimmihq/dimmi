/*
|--------------------------------------------------------------------------
| Push Devices
|--------------------------------------------------------------------------
|
| A browser a member enabled notifications on. It belongs to their user
| and to the browser session that registered it, so signing out of that
| browser stops its pushes. Only real push services over https are
| accepted, the subscription's secret is sealed, and a browser that
| registers again takes its old row over instead of adding a duplicate.
| A push service that reports a device gone has it removed at once.
|
*/

package notifications

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Devices manages users' push subscriptions. PublicKey is the install's
VAPID public key browsers subscribe with.
*/
type Devices struct {
	DB        *database.DB
	Clock     clock.Clock
	Keys      *encryption.Keys
	PublicKey string
}

/*
NewDevice is a browser's subscription: its push endpoint and keys, as
the browser gives them, with a label for the device list.
*/
type NewDevice struct {
	UserID    ids.UUID
	SessionID *ids.UUID
	Endpoint  string
	P256dh    string
	Auth      string
	Label     string
	UserAgent string
}

/*
ErrPushEndpoint refuses an endpoint that is not a push service.
*/
var ErrPushEndpoint = fault.New(fault.Invalid, "push_endpoint_invalid",
	"That is not a browser push subscription.").OnField("endpoint")

/*
KeyID names the VAPID key a subscription was made with.
*/
func (d *Devices) KeyID() string {
	return hashing.SHA256Hex([]byte(d.PublicKey))[:16]
}

/*
Register stores the browser's subscription.
*/
func (d *Devices) Register(ctx context.Context, in NewDevice) (models.PushSubscription, error) {
	platform, err := checkSubscription(in)
	if err != nil {
		return models.PushSubscription{}, err
	}
	hash := hashing.SHA256([]byte(in.Endpoint))
	sealed, err := encryption.Seal(ctx, d.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "system"}, AAD: pushAAD(hash),
	}, in.Auth)
	if err != nil {
		return models.PushSubscription{}, fmt.Errorf("seal push secret: %w", err)
	}
	row, err := models.New(d.DB).UpsertPushSubscription(ctx, models.UpsertPushSubscriptionParams{
		UserID: in.UserID, SessionID: in.SessionID, Endpoint: in.Endpoint, EndpointHash: hash, P256dh: in.P256dh,
		AuthSecretEncrypted: sealed, VapidKeyID: d.KeyID(), Platform: platform, UserAgent: clip(in.UserAgent, 512),
		DeviceLabel: clip(in.Label, 80), Now: d.Clock.Now(),
	})
	if err != nil {
		return row, fmt.Errorf("store push subscription: %w", err)
	}
	return row, nil
}

/*
List returns the user's devices.
*/
func (d *Devices) List(ctx context.Context, userID ids.UUID) ([]models.PushSubscription, error) {
	rows, err := models.New(d.DB).ListUserPushSubscriptions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	return rows, nil
}

/*
Remove deletes one of the user's devices.
*/
func (d *Devices) Remove(ctx context.Context, userID, id ids.UUID) error {
	n, err := models.New(d.DB).DeleteUserPushSubscription(ctx, models.DeleteUserPushSubscriptionParams{
		UserID: userID, ID: id,
	})
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	if n == 0 {
		return exceptions.ErrNotFound
	}
	return nil
}

/*
Gone removes a subscription the push service reports as gone.
*/
func (d *Devices) Gone(ctx context.Context, endpoint string) error {
	err := models.New(d.DB).DeletePushSubscriptionByEndpoint(ctx, hashing.SHA256([]byte(endpoint)))
	if err != nil {
		return fmt.Errorf("delete gone push subscription: %w", err)
	}
	return nil
}

/*
checkSubscription accepts only an https endpoint on a known push
service, with keys of the right sizes, and names the platform.
*/
func checkSubscription(in NewDevice) (string, error) {
	u, err := url.Parse(in.Endpoint)
	if err != nil || u.Scheme != "https" || len(in.Endpoint) > 2048 {
		return "", ErrPushEndpoint
	}
	platform, ok := pushPlatform(u.Hostname())
	if !ok {
		return "", ErrPushEndpoint
	}
	if size(in.P256dh) != 65 || size(in.Auth) != 16 {
		return "", fault.New(fault.Invalid, "push_keys_invalid", "The subscription's keys are not valid.").
			OnField("keys")
	}
	return platform, nil
}

/*
pushPlatform names the push service a host belongs to.
*/
func pushPlatform(host string) (string, bool) {
	switch {
	case host == "fcm.googleapis.com" || host == "android.googleapis.com":
		return "google", true
	case strings.HasSuffix(host, ".push.services.mozilla.com"):
		return "mozilla", true
	case strings.HasSuffix(host, ".notify.windows.com"):
		return "microsoft", true
	case host == "web.push.apple.com" || strings.HasSuffix(host, ".push.apple.com"):
		return "apple", true
	}
	return "", false
}

/*
size is how many bytes a base64url key holds, -1 when it is not one.
*/
func size(key string) int {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(key, "="))
	if err != nil {
		return -1
	}
	return len(raw)
}

func clip(s string, n int) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return &s
}
