/*
|--------------------------------------------------------------------------
| Sweep Assignment Queues
|--------------------------------------------------------------------------
|
| A conversation waiting for an assignee must not wait for ever because
| the thing that freed someone up was missed. Every minute this round
| queues a drain for each workspace with anything waiting; a drain
| already queued is not queued twice.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
SweepAssignmentQueuesArgs starts the round that drains every workspace
with waiting conversations.
*/
type SweepAssignmentQueuesArgs struct{}

/*
Kind names the job.
*/
func (SweepAssignmentQueuesArgs) Kind() string {
	return "sweep_assignment_queues"
}

/*
Options run the round on the maintenance queue.
*/
func (SweepAssignmentQueuesArgs) Options() queue.Options {
	return queue.Options{Queue: "maintenance"}
}

/*
SweepAssignmentQueues queues a drain for every workspace with waiting
conversations.
*/
type SweepAssignmentQueues struct {
	Assigner Assigner
	Queue    *queue.Queue
}

/*
Handle queues the drains.
*/
func (w *SweepAssignmentQueues) Handle(ctx context.Context, _ *queue.Job[SweepAssignmentQueuesArgs]) error {
	workspaces, err := w.Assigner.WaitingWorkspaces(ctx, waitingSweepSize)
	if err != nil {
		return err
	}

	for _, id := range workspaces {
		if err := w.Queue.Push(ctx, DrainAssignmentQueueArgs{WorkspaceID: id}); err != nil {
			return err
		}
	}

	return nil
}
