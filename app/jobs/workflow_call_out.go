/*
|--------------------------------------------------------------------------
| Workflow Call Out Jobs
|--------------------------------------------------------------------------
|
| A workflow step that calls the customer's system or sorts by topic
| does not hold the conversation while it waits for the answer: the run
| waits, and this job makes the call outside any transaction and hands
| the answer back. It is unique while pending, so a step queues it once.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
WorkflowCaller makes the slow calls of workflow steps.
*/
type WorkflowCaller interface {
	/*
		CallOut makes the call a run waits on at a slow step, outside
		the run's transaction, and resumes the run with the answer.
	*/
	CallOut(ctx context.Context, args CallOutForWorkflowArgs) error
}

/*
CallOutForWorkflowArgs names a run waiting at a slow step, such as a
request to the customer's system or sorting by topic, at the sequence
it waits on: the job makes the call outside the run's transaction and
hands the answer back.
*/
type CallOutForWorkflowArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RunID       ids.UUID `json:"run_id"`
	Seq         int64    `json:"seq"`
	StepID      string   `json:"step_id"`
}

/*
Kind names the job.
*/
func (CallOutForWorkflowArgs) Kind() string { return "call_out_for_workflow" }

/*
Options keep one pending job per run, sequence and step.
*/
func (CallOutForWorkflowArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
CallOutForWorkflow makes one slow step's call.
*/
type CallOutForWorkflow struct {
	Engine WorkflowCaller
}

/*
Handle makes the call.
*/
func (w *CallOutForWorkflow) Handle(ctx context.Context, job *queue.Job[CallOutForWorkflowArgs]) error {
	return w.Engine.CallOut(ctx, job.Args)
}
