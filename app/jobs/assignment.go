/*
|--------------------------------------------------------------------------
| Assignment Jobs
|--------------------------------------------------------------------------
|
| Automatic assignment runs off the request that caused it, so receiving
| a message stays quick and never fails because of a pick. One job
| assigns one conversation, at most once per reason while it waits; one
| drains a workspace's waiting conversations, never queued twice, so a
| burst of closes drains once; and a round every minute catches whatever
| a missed trigger left waiting.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Assigner gives conversations an assignee by their inbox's rule.
*/
type Assigner interface {
	/*
		AssignConversation assigns one conversation for the reason.
	*/
	AssignConversation(ctx context.Context, workspaceID, conversationID ids.UUID, reason string) error

	/*
		DrainWorkspace assigns the workspace's waiting conversations.
	*/
	DrainWorkspace(ctx context.Context, workspaceID ids.UUID) error

	/*
		WaitingWorkspaces returns the workspaces with waiting
		conversations, at most limit.
	*/
	WaitingWorkspaces(ctx context.Context, limit int32) ([]ids.UUID, error)
}

/*
waitingSweepSize is the most workspaces one sweep queues drains for.
*/
const waitingSweepSize = 1000

/*
AssignConversationArgs names the conversation to assign and why.
*/
type AssignConversationArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
	Reason         string   `json:"reason"`
}

/*
Kind names the job.
*/
func (AssignConversationArgs) Kind() string {
	return "assign_conversation"
}

/*
Options run the job once per conversation and reason while one waits.
*/
func (AssignConversationArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
AssignConversation assigns one conversation by its inbox's rule.
*/
type AssignConversation struct {
	Assigner Assigner
}

/*
Handle assigns the conversation.
*/
func (w *AssignConversation) Handle(ctx context.Context, job *queue.Job[AssignConversationArgs]) error {
	return w.Assigner.AssignConversation(ctx, job.Args.WorkspaceID, job.Args.ConversationID, job.Args.Reason)
}

/*
DrainAssignmentQueueArgs names the workspace whose waiting
conversations get another chance.
*/
type DrainAssignmentQueueArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
}

/*
Kind names the job.
*/
func (DrainAssignmentQueueArgs) Kind() string {
	return "drain_assignment_queue"
}

/*
Options keep one drain per workspace waiting at a time.
*/
func (DrainAssignmentQueueArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
DrainAssignmentQueue drains a workspace's waiting conversations.
*/
type DrainAssignmentQueue struct {
	Assigner Assigner
}

/*
Handle drains the workspace.
*/
func (w *DrainAssignmentQueue) Handle(ctx context.Context, job *queue.Job[DrainAssignmentQueueArgs]) error {
	return w.Assigner.DrainWorkspace(ctx, job.Args.WorkspaceID)
}

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

/*
pendingStates are the states in which a job counts as still to run.
*/
func pendingStates() []queue.State {
	return []queue.State{queue.StateAvailable, queue.StatePending, queue.StateScheduled, queue.StateRetryable}
}
