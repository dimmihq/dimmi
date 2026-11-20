/*
|--------------------------------------------------------------------------
| Process Social Event
|--------------------------------------------------------------------------
|
| A kept Messenger or Instagram delivery is processed by this job, apart
| from the request Meta made, so Meta hears back at once and a failure
| is tried again later. The job names the platform, whose channel then
| reads the delivery.
|
*/

package jobs

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
ProcessSocialEventArgs names the delivery to process and the platform
it came from.
*/
type ProcessSocialEventArgs struct {
	Platform    string   `json:"platform"`
	WorkspaceID ids.UUID `json:"workspace_id"`
	EventID     ids.UUID `json:"event_id"`
}

/*
Kind names the job.
*/
func (ProcessSocialEventArgs) Kind() string {
	return "process_social_event"
}

/*
Options try the job several times, once per delivery.
*/
func (ProcessSocialEventArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: channelEventAttempts, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
ProcessSocialEvent runs the processing of kept Messenger and Instagram
deliveries, each by its platform's channel.
*/
type ProcessSocialEvent struct {
	Processors map[string]ChannelEventProcessor
}

/*
Handle processes the delivery the job names.
*/
func (w *ProcessSocialEvent) Handle(ctx context.Context, job *queue.Job[ProcessSocialEventArgs]) error {
	processor, ok := w.Processors[job.Args.Platform]
	if !ok {
		return fmt.Errorf("process social event: no channel for platform %q", job.Args.Platform)
	}

	return processor.Process(ctx, job.Args.WorkspaceID, job.Args.EventID)
}
