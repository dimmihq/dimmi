/*
|--------------------------------------------------------------------------
| Operator Routine Delivery Job
|--------------------------------------------------------------------------
|
| Once a routine's run ends, a job of its own tells the people the
| routine reports to, retried while Slack or mail is down.
|
*/

package aijobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/operator"
)

/*
DeliverOperatorRoutineArgs names the run whose people to tell.
*/
type DeliverOperatorRoutineArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RunID       ids.UUID `json:"run_id" river:"unique"`
}

/*
Kind names the job.
*/
func (DeliverOperatorRoutineArgs) Kind() string { return "deliver_operator_routine" }

/*
Options tell each run once, retried when a service is down.
*/
func (DeliverOperatorRoutineArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 5, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
RoutineDeliverer tells a run's people how it went.
*/
type RoutineDeliverer interface {
	/*
		Deliver tells the run's people, once.
	*/
	Deliver(ctx context.Context, workspaceID, runID ids.UUID) error
}

/*
DeliverOperatorRoutine tells one run's people.
*/
type DeliverOperatorRoutine struct {
	Delivery RoutineDeliverer
}

/*
Handle tells them.
*/
func (w *DeliverOperatorRoutine) Handle(ctx context.Context, job *queue.Job[DeliverOperatorRoutineArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	return w.Delivery.Deliver(ctx, job.Args.WorkspaceID, job.Args.RunID)
}

/*
PushRoutineDelivery queues the telling of a run in the transaction that
ended it.
*/
func PushRoutineDelivery(q *queue.Queue) operator.PushRun {
	return func(ctx context.Context, tx *database.Tx, workspaceID, runID ids.UUID) error {
		return q.PushTx(ctx, tx, DeliverOperatorRoutineArgs{WorkspaceID: workspaceID, RunID: runID})
	}
}
