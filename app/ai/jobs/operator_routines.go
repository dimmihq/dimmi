/*
|--------------------------------------------------------------------------
| Operator Routine Jobs
|--------------------------------------------------------------------------
|
| Every minute the routines whose time came are queued; each run is a
| job of its own, which runs Operator for up to a few minutes. Runs wait
| behind the jobs that answer customers, so a long audit never holds up
| a reply.
|
*/

package aijobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/operator"
)

/*
QueueOperatorRoutinesArgs names the periodic job that queues the
routines whose time came; it takes no arguments.
*/
type QueueOperatorRoutinesArgs struct{}

/*
Kind names the job.
*/
func (QueueOperatorRoutinesArgs) Kind() string { return "queue_operator_routines" }

/*
Options run the round at most once a minute, once.
*/
func (QueueOperatorRoutinesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1, Unique: queue.UniqueOpts{ByPeriod: 50 * time.Second}}
}

/*
QueueOperatorRoutines queues the due routines' runs.
*/
type QueueOperatorRoutines struct {
	Operator *operator.Engine
	Queue    *queue.Queue
}

/*
Handle queues the runs.
*/
func (w *QueueOperatorRoutines) Handle(ctx context.Context, _ *queue.Job[QueueOperatorRoutinesArgs]) error {
	return w.Operator.QueueDueRoutines(ctx, PushRoutineRun(w.Queue))
}

/*
PushRoutineRun queues a run's job in the transaction that created it.
*/
func PushRoutineRun(q *queue.Queue) operator.PushRun {
	return func(ctx context.Context, tx *database.Tx, workspaceID, runID ids.UUID) error {
		return q.PushTx(ctx, tx, RunOperatorRoutineArgs{WorkspaceID: workspaceID, RunID: runID})
	}
}

/*
RunOperatorRoutineArgs names the run to carry out.
*/
type RunOperatorRoutineArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RunID       ids.UUID `json:"run_id" river:"unique"`
}

/*
Kind names the job.
*/
func (RunOperatorRoutineArgs) Kind() string { return "run_operator_routine" }

/*
Options try a run three times when the database fails it, behind the
jobs that answer customers.
*/
func (RunOperatorRoutineArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Priority: 3, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
RunOperatorRoutine carries out one run of a routine.
*/
type RunOperatorRoutine struct {
	Operator *operator.Engine
}

/*
Timeout gives a run longer than the queue's default: Operator works for
up to four minutes, and the run then stores what it found.
*/
func (w *RunOperatorRoutine) Timeout() time.Duration { return 6 * time.Minute }

/*
Handle runs it.
*/
func (w *RunOperatorRoutine) Handle(ctx context.Context, job *queue.Job[RunOperatorRoutineArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	return w.Operator.RunRoutine(ctx, job.Args.WorkspaceID, job.Args.RunID)
}
