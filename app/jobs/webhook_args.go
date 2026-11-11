/*
|--------------------------------------------------------------------------
| Webhook Job Arguments
|--------------------------------------------------------------------------
|
| What each webhook job carries and how the queue treats it. A fan-out
| is retried a few times, a send waits once per delivery and trigger,
| and the sweep, check and prune run once, leaving the rest to their
| next scheduled run.
|
*/

package jobs

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Attempts of the webhook jobs. A send is one request: its retries follow
the delivery's own schedule, never the queue's.
*/
const (
	fanOutAttempts  = 10
	deliverAttempts = 3
)

/*
FanOutWebhookEventArgs is one event to fan out to the workspace's
endpoints. Subject and SubjectID name the object its payload carries,
and Data the extra fields of the event, such as what changed. ActorType
and ActorID say who caused it.
*/
type FanOutWebhookEventArgs struct {
	WorkspaceID ids.UUID        `json:"workspace_id"`
	EventID     ids.UUID        `json:"event_id"`
	Type        string          `json:"type"`
	OccurredAt  time.Time       `json:"occurred_at"`
	ActorType   string          `json:"actor_type"`
	ActorID     *ids.UUID       `json:"actor_id,omitempty"`
	Subject     string          `json:"subject"`
	SubjectID   ids.UUID        `json:"subject_id"`
	Data        json.RawMessage `json:"data,omitempty"`
}

/*
Kind names the job.
*/
func (FanOutWebhookEventArgs) Kind() string { return "fan_out_webhook_event" }

/*
Options fan an event out once, retried a few times.
*/
func (FanOutWebhookEventArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: fanOutAttempts}
}

/*
DeliverWebhookArgs is one attempt to send one delivery. Trigger is
auto for the schedule and manual for a redelivery.
*/
type DeliverWebhookArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	DeliveryID  ids.UUID `json:"delivery_id"`
	Trigger     string   `json:"trigger"`
}

/*
Kind names the job.
*/
func (DeliverWebhookArgs) Kind() string { return "deliver_webhook" }

/*
Options keep one waiting send per delivery and trigger.
*/
func (DeliverWebhookArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: deliverAttempts,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRunning, queue.StateScheduled, queue.StateRetryable,
		}},
	}
}

/*
SweepWebhookDeliveriesArgs names the minute's sweep of due retries; it
takes no arguments.
*/
type SweepWebhookDeliveriesArgs struct{}

/*
Kind names the job.
*/
func (SweepWebhookDeliveriesArgs) Kind() string { return "sweep_webhook_deliveries" }

/*
Options run the sweep once; the next minute's run takes over.
*/
func (SweepWebhookDeliveriesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1}
}

/*
CheckWebhookEndpointsArgs names the check of failing endpoints and
expired secrets; it takes no arguments.
*/
type CheckWebhookEndpointsArgs struct{}

/*
Kind names the job.
*/
func (CheckWebhookEndpointsArgs) Kind() string { return "check_webhook_endpoints" }

/*
Options run the check once; the next run takes over.
*/
func (CheckWebhookEndpointsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1}
}

/*
PruneWebhookDeliveriesArgs names the nightly prune of the delivery log;
it takes no arguments.
*/
type PruneWebhookDeliveriesArgs struct{}

/*
Kind names the job.
*/
func (PruneWebhookDeliveriesArgs) Kind() string { return "prune_webhook_deliveries" }

/*
Options run the prune once a day at most, and retry it a few times.
*/
func (PruneWebhookDeliveriesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}
