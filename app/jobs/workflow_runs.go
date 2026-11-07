/*
|--------------------------------------------------------------------------
| Workflow Run Jobs
|--------------------------------------------------------------------------
|
| A run moves on from a job carrying the sequence it expects, so a job
| that arrives late finds nothing to do; the same job wakes a run whose
| timer is due. Customer messages resume the runs waiting for them.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
AdvanceWorkflowRunArgs names a run and the sequence it is expected at.
Wake says the job is the run's timer; Input names what else a waiting
run receives, such as a teammate's reply.
*/
type AdvanceWorkflowRunArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RunID       ids.UUID `json:"run_id"`
	Seq         int64    `json:"seq"`
	Wake        bool     `json:"wake,omitempty"`
	Input       string   `json:"input,omitempty"`
}

/*
Kind names the job.
*/
func (AdvanceWorkflowRunArgs) Kind() string { return "advance_workflow_run" }

/*
Options keep one job per run and sequence waiting.
*/
func (AdvanceWorkflowRunArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
AdvanceWorkflowRun moves one run on.
*/
type AdvanceWorkflowRun struct {
	Engine WorkflowEngine
}

/*
Handle moves the run on.
*/
func (w *AdvanceWorkflowRun) Handle(ctx context.Context, job *queue.Job[AdvanceWorkflowRunArgs]) error {
	return w.Engine.AdvanceRun(ctx, job.Args)
}

/*
ResumeWorkflowInputArgs names a customer message runs may wait for.
*/
type ResumeWorkflowInputArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
	MessageID      ids.UUID `json:"message_id"`
}

/*
Kind names the job.
*/
func (ResumeWorkflowInputArgs) Kind() string { return "resume_workflow_input" }

/*
Options keep one job per message.
*/
func (ResumeWorkflowInputArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ResumeWorkflowInput hands a customer message to waiting runs.
*/
type ResumeWorkflowInput struct {
	Engine WorkflowEngine
}

/*
Handle resumes the runs.
*/
func (w *ResumeWorkflowInput) Handle(ctx context.Context, job *queue.Job[ResumeWorkflowInputArgs]) error {
	return w.Engine.ResumeOnMessage(ctx, job.Args.WorkspaceID, job.Args.ConversationID, job.Args.MessageID)
}
