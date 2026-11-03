/*
|--------------------------------------------------------------------------
| Process Inbound Email
|--------------------------------------------------------------------------
|
| Received mail is recorded while the provider waits and processed here,
| afterwards: parsed, authenticated, sorted, threaded and handed to the
| inbox. A run that fails is tried again with growing pauses, since most
| failures pass, such as the database or a disk being briefly away; the
| email's row says what went wrong meanwhile.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
inboundEmailAttempts is how often processing one email is tried.
*/
const inboundEmailAttempts = 8

/*
ProcessInboundEmailArgs names the received email to process.
*/
type ProcessInboundEmailArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	InboundEmailID ids.UUID `json:"inbound_email_id"`
}

/*
Kind names the job.
*/
func (ProcessInboundEmailArgs) Kind() string {
	return "process_inbound_email"
}

/*
Options try the job several times, once per email.
*/
func (ProcessInboundEmailArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: inboundEmailAttempts}
}

/*
InboundEmailProcessor processes one received email.
*/
type InboundEmailProcessor interface {
	/*
		Handle processes the email of the workspace.
	*/
	Handle(ctx context.Context, workspaceID, inboundEmailID ids.UUID) error
}

/*
ProcessInboundEmail runs the processing of received emails.
*/
type ProcessInboundEmail struct {
	Processor InboundEmailProcessor
}

/*
Handle processes the email the job names.
*/
func (w *ProcessInboundEmail) Handle(ctx context.Context, job *queue.Job[ProcessInboundEmailArgs]) error {
	return w.Processor.Handle(ctx, job.Args.WorkspaceID, job.Args.InboundEmailID)
}
