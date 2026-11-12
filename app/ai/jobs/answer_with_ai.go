/*
|--------------------------------------------------------------------------
| Answer With AI
|--------------------------------------------------------------------------
|
| Carries a customer's new message to the pipeline, a beat after it
| landed, so rapid typing gathers into one reply: the job of every
| message but the newest steps aside. The job is unique per message, so
| a repeated event never queues it twice, and a retry finds the run
| that already decided its messages and never answers twice.
|
*/

package aijobs

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
)

/*
AnswerWithAIArgs is one trigger message to answer. Only the message id
makes a job unique; QueuedAt is in Unix milliseconds. Delayed marks a
job the channel held back so a teammate could answer first.
*/
type AnswerWithAIArgs struct {
	WorkspaceID      ids.UUID `json:"workspace_id"`
	ConversationID   ids.UUID `json:"conversation_id"`
	TriggerMessageID ids.UUID `json:"trigger_message_id" river:"unique"`
	RunKind          string   `json:"run_kind"`
	QueuedAt         int64    `json:"queued_at"`
	Delayed          bool     `json:"delayed,omitempty"`
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
		QueuedAt:         time.UnixMilli(job.Args.QueuedAt),
		Delayed:          job.Args.Delayed,
	})
	return waitInFlight(err)
}

/*
inFlightWait is how long a job waits for another run answering its
messages before it looks again.
*/
const inFlightWait = 2 * time.Second

/*
waitInFlight holds the job back while another run answers the same
messages, and passes any other error on.
*/
func waitInFlight(err error) error {
	if errors.Is(err, pipeline.ErrInFlight) {
		return queue.Postpone(inFlightWait)
	}
	return err
}
