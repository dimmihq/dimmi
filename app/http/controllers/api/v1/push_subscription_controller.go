/*
|--------------------------------------------------------------------------
| Push Subscription Controller
|--------------------------------------------------------------------------
|
| The browsers a person enabled push on: the key a browser subscribes
| with, the list of devices, adding this browser and removing one. They
| belong to the user, across workspaces, and never show their keys.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
PushSubscriptionController serves a user's push devices.
*/
type PushSubscriptionController struct {
	Devices *notifications.Devices
	IDs     *ids.Registry
}

/*
PushSubscriptionsOutput is the user's devices.
*/
type PushSubscriptionsOutput struct {
	Body fhttp.Collection[resources.PushSubscription]
}

/*
PushSubscriptionOutput is one device.
*/
type PushSubscriptionOutput struct {
	Body fhttp.Resource[resources.PushSubscription]
}

/*
PushConfigOutput is what a browser subscribes with.
*/
type PushConfigOutput struct {
	Body fhttp.Resource[resources.PushConfig]
}

/*
Config returns the install's push key.
*/
func (c *PushSubscriptionController) Config(context.Context, *requests.NoInput) (*PushConfigOutput, error) {
	return &PushConfigOutput{Body: fhttp.NewResource(resources.PushConfig{
		VAPIDPublicKey: c.Devices.PublicKey, KeyID: c.Devices.KeyID(),
	})}, nil
}

/*
Index lists the user's devices.
*/
func (c *PushSubscriptionController) Index(ctx context.Context, _ *requests.NoInput,
) (*PushSubscriptionsOutput, error) {
	rows, err := c.Devices.List(ctx, caller(ctx).User.ID)
	if err != nil {
		return nil, err
	}
	items := make([]resources.PushSubscription, len(rows))
	for i, r := range rows {
		items[i] = resources.NewPushSubscription(r)
	}
	page := pagination.Page[resources.PushSubscription]{Items: items, Limit: len(items)}
	return &PushSubscriptionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store registers this browser for push.
*/
func (c *PushSubscriptionController) Store(ctx context.Context, in *requests.StorePushSubscriptionRequest,
) (*PushSubscriptionOutput, error) {
	p, r := caller(ctx), requestOf(ctx)
	device := notifications.NewDevice{
		UserID: p.User.ID, Endpoint: in.Body.Endpoint, P256dh: in.Body.Keys.P256dh, Auth: in.Body.Keys.Auth,
		Label: in.Body.DeviceLabel,
	}
	if p.Kind == "session" {
		device.SessionID = &p.CredentialID
	}
	if r != nil {
		device.UserAgent = r.UserAgent()
	}
	row, err := c.Devices.Register(ctx, device)
	if err != nil {
		return nil, err
	}
	return &PushSubscriptionOutput{Body: fhttp.NewResource(resources.NewPushSubscription(row))}, nil
}

/*
Destroy removes one of the user's devices.
*/
func (c *PushSubscriptionController) Destroy(ctx context.Context, in *requests.PushSubscriptionRequest,
) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixPushSubscription), in.Subscription)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	return &EmptyOutput{}, c.Devices.Remove(ctx, caller(ctx).User.ID, id)
}

/*
requestOf returns the HTTP request in the context.
*/
func requestOf(ctx context.Context) *fhttp.Request {
	r, _ := fhttp.RequestFrom(ctx)
	return r
}
