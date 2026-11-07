/*
|--------------------------------------------------------------------------
| Gone Push Subscriptions
|--------------------------------------------------------------------------
|
| The push channel is built before the application boots, so it reports
| a browser the push service no longer knows through this method, which
| reaches the device actions once they exist.
|
*/

package app

import (
	"context"

	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
)

/*
PushGone removes a push subscription the push service reports as gone.
*/
func (a *App) PushGone(ctx context.Context, _ fnotify.Delivery, sub webpush.Subscription) error {
	if a.Actions == nil || a.Actions.PushDevices == nil {
		return nil
	}
	return a.Actions.PushDevices.Gone(ctx, sub.Endpoint)
}
