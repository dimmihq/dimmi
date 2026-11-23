/*
|--------------------------------------------------------------------------
| Routines Coming Due
|--------------------------------------------------------------------------
|
| Every minute the routines whose time came are queued, a bounded number
| a round. Each is locked, checked again and moved on to its next run in
| the same transaction that queues the run, so a routine runs once per
| slot however many nodes sweep. A routine still running from before
| skips the slot instead of piling runs up; a run that stopped moving,
| a worker lost mid-run, is closed as stalled first. Missed slots are not
| caught up: the routine runs once and moves to its next slot from now.
|
*/

package operator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cadence"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The sweep's bounds: routines queued a round, and how long a run may go
without moving before it counts as stalled.
*/
const (
	dueBatch     = 100
	stalledAfter = 20 * time.Minute
)

/*
PushRun queues the job that runs a routine's run, in the transaction
that created the run.
*/
type PushRun func(ctx context.Context, tx *database.Tx, workspaceID, runID ids.UUID) error

/*
ScheduleOf reads a routine's cadence.
*/
func ScheduleOf(r models.OperatorRoutine) (*cadence.Schedule, error) {
	return cadence.Parse(cadence.Spec{Kind: r.Cadence, Expr: deref(r.CronExpr), RunAt: r.RunAt, Timezone: r.Timezone})
}

/*
QueueDueRoutines queues the runs of the routines whose time came.
*/
func (e *Engine) QueueDueRoutines(ctx context.Context, push PushRun) error {
	e.prepare()
	due, err := models.New(e.DB).OperatorRoutinesDue(ctx, models.OperatorRoutinesDueParams{
		At: e.Clock.Now(), Batch: dueBatch,
	})
	if err != nil {
		return fmt.Errorf("list due routines: %w", err)
	}
	var errs []error
	for _, d := range due {
		scoped := tenancy.With(ctx, d.WorkspaceID.String())
		err := e.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			return e.queueDue(ctx, tx, runRef{workspace: d.WorkspaceID, run: d.ID}, push)
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

/*
queueDue queues one due routine's run and moves it on; the reference
names the routine, not a run.
*/
func (e *Engine) queueDue(ctx context.Context, tx *database.Tx, due runRef, push PushRun) error {
	q := models.New(tx)
	now := e.Clock.Now()
	r, err := q.LockOperatorRoutine(ctx, models.LockOperatorRoutineParams{WorkspaceID: due.workspace, ID: due.run})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock routine: %w", err)
	}
	if r.Status != "active" || r.NextRunAt == nil || r.NextRunAt.After(now) {
		return nil
	}
	life := lifecycleOf(r)
	sched, err := ScheduleOf(r)
	if err != nil {
		life.status, life.pause, life.next = "paused", new("invalid_schedule"), nil
		return e.saveLifecycle(ctx, q, r, life)
	}
	slot, workspaceID := *r.NextRunAt, r.WorkspaceID
	life.next = nextAfter(sched, now)
	busy, err := e.closeStalled(ctx, q, r, now)
	if err != nil {
		return err
	}
	params := models.CreateOperatorRoutineRunParams{
		WorkspaceID: workspaceID, RoutineID: r.ID, Trigger: TriggerScheduled, ScheduledFor: &slot,
		Status: "queued", Now: now,
	}
	if busy {
		params.Status, params.SkipReason, params.Outcome = "skipped", new("overlap_active_run"), new(RunSkipped)
		params.CompletedAt = &now
	}
	run, err := q.CreateOperatorRoutineRun(ctx, params)
	if err != nil {
		return fmt.Errorf("queue routine run: %w", err)
	}
	if err := e.saveLifecycle(ctx, q, r, life); err != nil || busy {
		return err
	}
	return push(ctx, tx, workspaceID, run.ID)
}

/*
nextAfter is the routine's next run after now, none when it has none.
*/
func nextAfter(s *cadence.Schedule, now time.Time) *time.Time {
	next := s.Next(now)
	if next.IsZero() {
		return nil
	}
	return &next
}

/*
NextRun is when a routine first runs from now, none when it has no run
left.
*/
func NextRun(r models.OperatorRoutine, now time.Time) (*time.Time, error) {
	s, err := ScheduleOf(r)
	if err != nil {
		return nil, err
	}
	return nextAfter(s, now), nil
}

/*
closeStalled closes the routine's live run when it stopped moving, and
reports whether one is still going.
*/
func (e *Engine) closeStalled(ctx context.Context, q *models.Queries, r models.OperatorRoutine, now time.Time,
) (bool, error) {
	run, err := q.FindActiveOperatorRoutineRun(ctx, models.FindActiveOperatorRoutineRunParams{
		WorkspaceID: r.WorkspaceID, RoutineID: r.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find live routine run: %w", err)
	}
	if now.Sub(run.UpdatedAt) < stalledAfter {
		return true, nil
	}
	run.Status, run.Outcome, run.ErrorCode, run.CompletedAt = "failed", new(RunFailed), new("stalled"), &now
	run.ErrorMessage = new("The run stopped making progress.")
	return false, e.saveRun(ctx, q, run)
}
