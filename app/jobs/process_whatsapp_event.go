/*
|--------------------------------------------------------------------------
| Process WhatsApp Event
|--------------------------------------------------------------------------
|
| A WhatsApp delivery is kept while Meta waits for its answer and
| processed here, afterwards: customers' messages become the inbox's,
| status reports move the business's messages along. A run that fails is
| tried again with growing pauses; the delivery's row says what went
| wrong meanwhile and never blocks the deliveries after it.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
ProcessWhatsAppEventArgs names the delivery to process.
*/
type ProcessWhatsAppEventArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	EventID     ids.UUID `json:"event_id"`
}

/*
Kind names the job.
*/
func (ProcessWhatsAppEventArgs) Kind() string {
	return "process_whatsapp_event"
}

/*
Options try the job several times, once per delivery.
*/
func (ProcessWhatsAppEventArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: channelEventAttempts, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
ProcessWhatsAppEvent runs the processing of kept WhatsApp deliveries.
*/
type ProcessWhatsAppEvent struct {
	Processor ChannelEventProcessor
}

/*
Handle processes the delivery the job names.
*/
func (w *ProcessWhatsAppEvent) Handle(ctx context.Context, job *queue.Job[ProcessWhatsAppEventArgs]) error {
	return w.Processor.Process(ctx, job.Args.WorkspaceID, job.Args.EventID)
}
