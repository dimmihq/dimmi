/*
|--------------------------------------------------------------------------
| Process Channel Inbound Event
|--------------------------------------------------------------------------
|
| A chat app's delivery is kept while the provider waits for its answer
| and processed here, afterwards. A run that fails is tried again with
| growing pauses, since most failures pass, such as the provider being
| briefly away; the delivery's row says what went wrong meanwhile and
| never blocks the deliveries after it.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
channelEventAttempts is how often processing one delivery is tried.
*/
const channelEventAttempts = 10

/*
ProcessChannelInboundEventArgs names the delivery to process.
*/
type ProcessChannelInboundEventArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	EventID     ids.UUID `json:"event_id"`
}

/*
Kind names the job.
*/
func (ProcessChannelInboundEventArgs) Kind() string {
	return "process_channel_inbound_event"
}

/*
Options try the job several times, once per delivery.
*/
func (ProcessChannelInboundEventArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: channelEventAttempts, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
ChannelEventProcessor processes one kept delivery.
*/
type ChannelEventProcessor interface {
	/*
		Process processes the delivery of the workspace.
	*/
	Process(ctx context.Context, workspaceID, eventID ids.UUID) error
}

/*
ProcessChannelInboundEvent runs the processing of kept deliveries.
*/
type ProcessChannelInboundEvent struct {
	Processor ChannelEventProcessor
}

/*
Handle processes the delivery the job names.
*/
func (w *ProcessChannelInboundEvent) Handle(ctx context.Context, job *queue.Job[ProcessChannelInboundEventArgs]) error {
	return w.Processor.Process(ctx, job.Args.WorkspaceID, job.Args.EventID)
}
