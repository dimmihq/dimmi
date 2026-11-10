/*
|--------------------------------------------------------------------------
| Send Notification Digests
|--------------------------------------------------------------------------
|
| Every minute the sweep sends the Slack digests whose time came, each at
| the time its channel picked in the workspace's zone. A digest is moved
| to its next day before it posts, so a run that fails halfway never
| posts the same digest twice.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
SendNotificationDigestsArgs carries nothing: every run sends everything
due.
*/
type SendNotificationDigestsArgs struct{}

/*
Kind names the job.
*/
func (SendNotificationDigestsArgs) Kind() string {
	return "send_notification_digests"
}

/*
Options run the sweep once; the next minute catches up.
*/
func (SendNotificationDigestsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1}
}

/*
DigestSender sends every digest that is due.
*/
type DigestSender interface {
	/*
		Due sends the digests whose time came.
	*/
	Due(ctx context.Context) error
}

/*
SendNotificationDigests sends the digests that are due.
*/
type SendNotificationDigests struct {
	Digests DigestSender
}

/*
Handle runs the sweep.
*/
func (w *SendNotificationDigests) Handle(ctx context.Context, _ *queue.Job[SendNotificationDigestsArgs]) error {
	return w.Digests.Due(ctx)
}
