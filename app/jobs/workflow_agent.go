/*
|--------------------------------------------------------------------------
| Workflow Agent Jobs
|--------------------------------------------------------------------------
|
| A workflow step that lets dimmi handle a conversation wakes the agent
| for the customer message already waiting, and hears back when the
| agent hands the conversation over. Both jobs are unique while
| pending, so a retried step never queues the agent twice.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
AnswerForWorkflowArgs asks the agent to answer the customer message a
run's agent step found waiting.
*/
type AnswerForWorkflowArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
	MessageID      ids.UUID `json:"message_id"`
	RunID          ids.UUID `json:"run_id"`
}

/*
Kind names the job.
*/
func (AnswerForWorkflowArgs) Kind() string { return "answer_for_workflow" }

/*
Options keep one pending job per run and message.
*/
func (AnswerForWorkflowArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ResumeWorkflowOnAgentArgs tells the runs of a conversation that the
agent handed it over, and why.
*/
type ResumeWorkflowOnAgentArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
	Reason         string   `json:"reason"`
	EventID        ids.UUID `json:"event_id"`
}

/*
Kind names the job.
*/
func (ResumeWorkflowOnAgentArgs) Kind() string { return "resume_workflow_on_agent" }

/*
Options keep one pending job per handover.
*/
func (ResumeWorkflowOnAgentArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ResumeWorkflowOnAgent hands the agent's handover to the runs waiting on
it.
*/
type ResumeWorkflowOnAgent struct {
	Engine WorkflowEngine
}

/*
Handle resumes the runs.
*/
func (w *ResumeWorkflowOnAgent) Handle(ctx context.Context, job *queue.Job[ResumeWorkflowOnAgentArgs]) error {
	return w.Engine.ResumeOnAgent(ctx, job.Args.WorkspaceID, job.Args.ConversationID, job.Args.Reason)
}
