/*
|--------------------------------------------------------------------------
| Quiet Trigger Jobs
|--------------------------------------------------------------------------
|
| Silence has no event of its own, so the quiet triggers run on timers.
| A reply that could start a silence arms one timer per live workflow
| listening for it; the timer fires the trigger for that workflow only
| if the silence still holds when it comes due. Both jobs are unique
| while pending, so a repeated event never arms twice.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
ArmQuietTriggersArgs names the message that may start a silence: a
teammate's or the agent's reply for a quiet customer, a customer's
message for a customer left waiting.
*/
type ArmQuietTriggersArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
	MessageID      ids.UUID `json:"message_id"`
	Trigger        string   `json:"trigger"`
	Author         string   `json:"author"`
}

/*
Kind names the job.
*/
func (ArmQuietTriggersArgs) Kind() string { return "arm_quiet_triggers" }

/*
Options keep one pending job per message and trigger.
*/
func (ArmQuietTriggersArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
FireQuietTriggerArgs is one workflow's quiet timer: the silence it
waits out, started by the anchor message or the moment the customer
began waiting, and when it is due.
*/
type FireQuietTriggerArgs struct {
	WorkspaceID    ids.UUID  `json:"workspace_id"`
	ConversationID ids.UUID  `json:"conversation_id"`
	WorkflowID     ids.UUID  `json:"workflow_id"`
	Trigger        string    `json:"trigger"`
	AnchorID       ids.UUID  `json:"anchor_id"`
	AnchorAt       time.Time `json:"anchor_at"`
	Due            time.Time `json:"due"`
}

/*
Kind names the job.
*/
func (FireQuietTriggerArgs) Kind() string { return "fire_quiet_trigger" }

/*
Options keep one pending timer per workflow and anchor.
*/
func (FireQuietTriggerArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
QuietEngine arms and fires the quiet triggers.
*/
type QuietEngine interface {
	/*
		ArmQuiet arms the timers a message starts.
	*/
	ArmQuiet(ctx context.Context, args ArmQuietTriggersArgs) error

	/*
		FireQuiet fires a timer's trigger when its silence still holds.
	*/
	FireQuiet(ctx context.Context, args FireQuietTriggerArgs) error
}

/*
ArmQuietTriggers arms the quiet timers of a message.
*/
type ArmQuietTriggers struct {
	Engine QuietEngine
}

/*
Handle arms the timers.
*/
func (w *ArmQuietTriggers) Handle(ctx context.Context, job *queue.Job[ArmQuietTriggersArgs]) error {
	return w.Engine.ArmQuiet(ctx, job.Args)
}

/*
FireQuietTrigger fires one quiet timer.
*/
type FireQuietTrigger struct {
	Engine QuietEngine
}

/*
Handle fires the timer.
*/
func (w *FireQuietTrigger) Handle(ctx context.Context, job *queue.Job[FireQuietTriggerArgs]) error {
	return w.Engine.FireQuiet(ctx, job.Args)
}
