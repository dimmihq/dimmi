/*
|--------------------------------------------------------------------------
| Answer For A Workflow
|--------------------------------------------------------------------------
|
| A workflow step that lets dimmi handle a conversation may begin after
| the customer's message already arrived, when the agent had stepped
| aside because the workflow held the conversation. This job hands
| that waiting message to the agent once the step is active; the
| pipeline still refuses a message it already answered.
|
*/

package aijobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
AnswerForWorkflow runs the agent on the message a workflow step found
waiting.
*/
type AnswerForWorkflow struct {
	Runner *pipeline.Runner
}

/*
Handle runs the pipeline for the waiting message.
*/
func (w *AnswerForWorkflow) Handle(ctx context.Context, job *queue.Job[jobs.AnswerForWorkflowArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	_, err := w.Runner.Run(ctx, pipeline.Args{
		WorkspaceID: job.Args.WorkspaceID, ConversationID: job.Args.ConversationID,
		TriggerMessageID: job.Args.MessageID, Kind: "conversation", QueuedAt: w.Runner.Clock.Now(),
	})

	return err
}
