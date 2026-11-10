/*
|--------------------------------------------------------------------------
| Broadcast Workers
|--------------------------------------------------------------------------
|
| The queue's way into the broadcast jobs. The hourly sweep restarts any
| scheduled or sending broadcast whose job was lost, so a crash or a
| flushed queue delays a send but never strands it.
|
*/

package broadcasts

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StartWorker runs StartBroadcast jobs.
*/
type StartWorker struct {
	Broadcasts *Broadcasts
}

/*
Handle starts the broadcast.
*/
func (w *StartWorker) Handle(ctx context.Context, job *queue.Job[jobs.StartBroadcastArgs]) error {
	return w.Broadcasts.Start(ctx, job.Args.WorkspaceID, job.Args.BroadcastID)
}

/*
DispatchWorker runs DispatchBroadcast jobs.
*/
type DispatchWorker struct {
	Broadcasts *Broadcasts
}

/*
Handle dispatches the broadcast's due recipients.
*/
func (w *DispatchWorker) Handle(ctx context.Context, job *queue.Job[jobs.DispatchBroadcastArgs]) error {
	return w.Broadcasts.Dispatch(ctx, job.Args.WorkspaceID, job.Args.BroadcastID)
}

/*
SendWorker runs SendBroadcastRecipient jobs.
*/
type SendWorker struct {
	Broadcasts *Broadcasts
}

/*
Handle reaches the recipient.
*/
func (w *SendWorker) Handle(ctx context.Context, job *queue.Job[jobs.SendBroadcastRecipientArgs]) error {
	return sendJob(w.Broadcasts.SendRecipient(ctx, job.Args.WorkspaceID, job.Args.RecipientID))
}

/*
TallyWorker runs TallyBroadcast jobs.
*/
type TallyWorker struct {
	Broadcasts *Broadcasts
}

/*
Handle counts the broadcast again.
*/
func (w *TallyWorker) Handle(ctx context.Context, job *queue.Job[jobs.TallyBroadcastArgs]) error {
	return w.Broadcasts.Tally(ctx, job.Args.WorkspaceID, job.Args.BroadcastID)
}

/*
SweepWorker runs the hourly sweep.
*/
type SweepWorker struct {
	Broadcasts *Broadcasts
}

/*
Handle restarts the active broadcasts of every workspace and checks
their goals.
*/
func (w *SweepWorker) Handle(ctx context.Context, _ *queue.Job[jobs.SweepBroadcastsArgs]) error {
	workspaces, err := models.New(w.Broadcasts.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to sweep: %w", err)
	}

	for _, workspace := range workspaces {
		if err := w.Broadcasts.sweep(ctx, workspace.ID); err != nil {
			return err
		}
	}

	return nil
}
