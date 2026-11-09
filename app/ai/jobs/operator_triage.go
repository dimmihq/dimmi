/*
|--------------------------------------------------------------------------
| Operator Triage Job
|--------------------------------------------------------------------------
|
| Operator looks at a conversation a customer started a little after it
| arrived, so the first messages are there to read. One job per
| conversation; a workspace that does not want triage skips it cheaply.
|
*/

package aijobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/operator"
)

/*
TriageWithOperatorArgs names the conversation to triage.
*/
type TriageWithOperatorArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id" river:"unique"`
}

/*
Kind names the job.
*/
func (TriageWithOperatorArgs) Kind() string { return "triage_with_operator" }

/*
TriageWithOperator runs Operator's triage on one conversation.
*/
type TriageWithOperator struct {
	Operator *operator.Engine
}

/*
Handle runs the triage.
*/
func (w *TriageWithOperator) Handle(ctx context.Context, job *queue.Job[TriageWithOperatorArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	return w.Operator.Triage(ctx, job.Args.WorkspaceID, job.Args.ConversationID)
}
