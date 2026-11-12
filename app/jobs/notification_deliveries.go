/*
|--------------------------------------------------------------------------
| Notification Delivery Upkeep
|--------------------------------------------------------------------------
|
| Every night the delivery log forgets what is older than thirty days.
| Every five minutes the quiet hours sweep sends a summary to each
| teammate whose quiet hours held email back and are now over.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
PruneNotificationDeliveriesArgs carries nothing: every run prunes
everything past retention.
*/
type PruneNotificationDeliveriesArgs struct{}

/*
Kind names the job.
*/
func (PruneNotificationDeliveriesArgs) Kind() string { return "prune_notification_deliveries" }

/*
Options run the prune once; the next night catches up.
*/
func (PruneNotificationDeliveriesArgs) Options() queue.Options { return queue.Options{MaxAttempts: 1} }

/*
SendQuietHoursSummariesArgs carries nothing: every run sends every
summary that is due.
*/
type SendQuietHoursSummariesArgs struct{}

/*
Kind names the job.
*/
func (SendQuietHoursSummariesArgs) Kind() string { return "send_quiet_hours_summaries" }

/*
Options run the sweep once; the next one catches up.
*/
func (SendQuietHoursSummariesArgs) Options() queue.Options { return queue.Options{MaxAttempts: 1} }

/*
DeliveryPruner deletes old delivery log lines.
*/
type DeliveryPruner interface {
	/*
		Prune deletes the lines past retention.
	*/
	Prune(ctx context.Context) error
}

/*
SummarySender sends the quiet hours summaries that are due.
*/
type SummarySender interface {
	/*
		Due sends every summary that is due.
	*/
	Due(ctx context.Context) error
}

/*
PruneNotificationDeliveries runs the nightly prune.
*/
type PruneNotificationDeliveries struct{ Log DeliveryPruner }

/*
Handle prunes the log.
*/
func (w *PruneNotificationDeliveries) Handle(ctx context.Context, _ *queue.Job[PruneNotificationDeliveriesArgs]) error {
	return w.Log.Prune(ctx)
}

/*
SendQuietHoursSummaries runs the summary sweep.
*/
type SendQuietHoursSummaries struct{ Summaries SummarySender }

/*
Handle sends the summaries that are due.
*/
func (w *SendQuietHoursSummaries) Handle(ctx context.Context, _ *queue.Job[SendQuietHoursSummariesArgs]) error {
	return w.Summaries.Due(ctx)
}
