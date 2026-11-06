/*
|--------------------------------------------------------------------------
| Answer With AI
|--------------------------------------------------------------------------
|
| Carries a customer's new message to the pipeline, a beat after it
| landed, so rapid typing gathers into one reply. The job is unique
| per trigger message: the debounce never doubles a run.
|
*/

package aijobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
AnswerWithAIArgs is one trigger message to answer.
*/
type AnswerWithAIArgs struct {
	WorkspaceID      ids.UUID `json:"workspace_id"`
	ConversationID   ids.UUID `json:"conversation_id"`
	TriggerMessageID ids.UUID `json:"trigger_message_id"`
	RunKind          string   `json:"run_kind"`
	QueuedAt         int64    `json:"queued_at"`
}

/*
Kind names the job.
*/
func (AnswerWithAIArgs) Kind() string { return "answer_with_ai" }

/*
AnswerWithAI runs the pipeline for one trigger message.
*/
type AnswerWithAI struct {
	Runner *pipeline.Runner
}

/*
Handle runs the pipeline.
*/
func (w *AnswerWithAI) Handle(ctx context.Context, job *queue.Job[AnswerWithAIArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())

	_, err := w.Runner.Run(ctx, pipeline.Args{
		WorkspaceID:      job.Args.WorkspaceID,
		ConversationID:   job.Args.ConversationID,
		TriggerMessageID: job.Args.TriggerMessageID,
		Kind:             job.Args.RunKind,
		QueuedAt:         time.Unix(job.Args.QueuedAt, 0),
	})
	return err
}
