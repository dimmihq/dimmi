/*
|--------------------------------------------------------------------------
| Advancing A Journey
|--------------------------------------------------------------------------
|
| A job takes one enrollment as far as it can go now. It first checks
| whether the traveler must leave or reached the goal, then runs steps
| until one waits, a path ends or the job has done its share. Every step
| leaves a row and moves the sequence on in the same transaction as its
| effect, so a job that carries an old sequence finds nothing to do and
| a retry never repeats a step. A waiting step is asked again each time
| something it may wait on changes, and when its time is up.
|
*/

package series

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
stepsPerJob is how many steps one job takes before it hands the rest to
another, keeping transactions short.
*/
const stepsPerJob = 25

/*
outcome is how a step went: the path to follow, a wait to hold on, or
the end of the journey, with what the trace records.
*/
type outcome struct {
	path    string
	wait    *wait
	end     string
	stay    bool
	result  string
	status  string
	details map[string]any
}

/*
wait is what a waiting step holds on: its kind, the step row it
belongs to, until when, and what a call made outside learned.
*/
type wait struct {
	Kind   string    `json:"kind"`
	StepID string    `json:"step_id"`
	Until  time.Time `json:"until,omitzero"`
	Result string    `json:"result,omitempty"`
}

/*
Advance moves one enrollment on.
*/
func (s *Series) Advance(ctx context.Context, a jobs.AdvanceSeriesEnrollmentArgs) error {
	return s.inTx(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		e, err := q.LockSeriesEnrollment(ctx, models.LockSeriesEnrollmentParams{
			WorkspaceID: a.WorkspaceID, ID: a.EnrollmentID,
		})
		if err != nil {
			return gone(err, "lock enrollment")
		}

		if ended(e) || (a.Seq != 0 && a.Seq != e.Seq) {
			return nil
		}

		row, err := find(ctx, q, e.WorkspaceID, e.SeriesID)
		if err != nil || row.Status != StatusLive {
			return gone(err, "find series")
		}

		r, err := s.open(ctx, tx, row, e)
		if err != nil {
			return gone(err, "open journey")
		}

		r.timer = a.Seq != 0

		return r.advance(ctx)
	})
}

/*
advance runs the journey's steps.
*/
func (r *run) advance(ctx context.Context) error {
	if left, err := r.guard(ctx); left || err != nil {
		return err
	}

	for range stepsPerJob {
		done, err := r.step(ctx)
		if done || err != nil {
			return err
		}
	}

	moved, err := r.q.MoveSeriesEnrollment(ctx, r.moveParams())
	if err != nil {
		return wrap("move journey", err)
	}

	return r.s.Queue.PushTx(ctx, r.tx, jobs.AdvanceSeriesEnrollmentArgs{
		WorkspaceID: moved.WorkspaceID, EnrollmentID: moved.ID, Seq: moved.Seq,
	})
}

/*
step runs or asks again the step the journey stands on; done is true
when the job should stop.
*/
func (r *run) step(ctx context.Context) (bool, error) {
	if int(r.e.StepsExecuted)+r.steps >= max(r.s.Config.MaxStepsPerEnrollment, 1) {
		return true, r.end(ctx, "step_budget")
	}

	n, ok := r.g.node(r.e.NodeID)
	if !ok {
		return true, r.end(ctx, "node_removed")
	}

	if r.e.Status == "waiting" {
		w := r.waitOf()

		out, err := r.resume(ctx, n, w)
		if err != nil {
			return true, err
		}

		if out.stay {
			return true, r.rearm(ctx, w)
		}

		return r.apply(ctx, n, w.StepID, out)
	}

	st, err := r.q.StartSeriesStep(ctx, models.StartSeriesStepParams{
		WorkspaceID: r.e.WorkspaceID, SeriesID: r.e.SeriesID, EnrollmentID: r.e.ID, VersionID: r.e.VersionID,
		NodeID: n.ID, Kind: n.Kind, Test: r.e.Test, Now: r.now,
	})
	if err != nil {
		return true, wrap("start step", err)
	}

	r.steps++

	out, err := r.enter(ctx, n, st.ID.String())
	if err != nil {
		return true, err
	}

	return r.apply(ctx, n, st.ID.String(), out)
}

/*
apply records how a step went and moves the journey on.
*/
func (r *run) apply(ctx context.Context, n Node, stepID string, out outcome) (bool, error) {
	if out.wait != nil {
		out.wait.StepID = stepID

		return true, r.hold(ctx, *out.wait)
	}

	if err := r.finishStep(ctx, stepID, out); err != nil {
		return true, err
	}

	if out.end != "" {
		return true, r.end(ctx, out.end)
	}

	to, ok := r.g.target(n.ID, out.path)
	if !ok {
		return true, r.end(ctx, "finished")
	}

	r.e.NodeID, r.e.Status, r.e.Wait, r.e.WakeAt = to, "active", nil, nil

	return false, nil
}

/*
hold stores the wait and, when it has a time, the job that ends it.
*/
func (r *run) hold(ctx context.Context, w wait) error {
	raw, _ := json.Marshal(w)
	p := r.moveParams()
	p.Status, p.Wait, p.WakeAt = "waiting", raw, nil

	if !w.Until.IsZero() {
		p.WakeAt = &w.Until
	}

	moved, err := r.q.MoveSeriesEnrollment(ctx, p)
	if err != nil {
		return wrap("hold journey", err)
	}

	r.e = moved

	if w.Kind == "call" {
		if err := r.s.Queue.PushTx(ctx, r.tx, jobs.CallSeriesStepArgs{
			WorkspaceID: moved.WorkspaceID, EnrollmentID: moved.ID, Seq: moved.Seq,
		}); err != nil {
			return wrap("queue step call", err)
		}
	}

	if w.Until.IsZero() {
		return nil
	}

	return r.s.Queue.PushTx(ctx, r.tx, jobs.AdvanceSeriesEnrollmentArgs{
		WorkspaceID: moved.WorkspaceID, EnrollmentID: moved.ID, Seq: moved.Seq,
	}, queue.At(w.Until))
}

/*
rearm puts the wait's timer off until its time when it fired early, as
a clock that runs behind the database's makes it do. Nothing changed,
so the job's transaction has nothing to keep.
*/
func (r *run) rearm(_ context.Context, w wait) error {
	if !r.timer || w.Until.IsZero() || !r.now.Before(w.Until) {
		return nil
	}

	return queue.Postpone(w.Until.Sub(r.now))
}

/*
end closes the journey for a reason: finished at a path's end,
anything else an exit. The step it waited on, if any, is closed too,
and what it had not sent yet is withdrawn.
*/
func (r *run) end(ctx context.Context, reason string) error {
	if r.e.Status == "waiting" {
		if err := r.finishStep(ctx, r.waitOf().StepID, outcome{
			status: "skipped",
			result: "left:" + reason,
		}); err != nil {
			return err
		}
	}

	p := r.moveParams()
	p.Status, p.Wait, p.WakeAt, p.LeftAt, p.EndReason = "exited", nil, nil, &r.now, &reason

	if reason == "finished" {
		p.Status = "finished"
	}

	moved, err := r.q.MoveSeriesEnrollment(ctx, p)
	if err != nil {
		return wrap("end journey", err)
	}

	r.e = moved

	if err := r.s.withdraw(ctx, r.q, moved); err != nil {
		return err
	}

	return r.ended(ctx, reason)
}

/*
finishStep records how a step ended.
*/
func (r *run) finishStep(ctx context.Context, stepID string, out outcome) error {
	id, err := parseID(stepID)
	if err != nil {
		return fmt.Errorf("read step id: %w", err)
	}

	status := out.status
	if status == "" {
		status = "done"
	}

	detail, _ := json.Marshal(out.details)
	if out.details == nil {
		detail = []byte("{}")
	}

	path := ""
	if out.end == "" && out.status == "" {
		path = pathOrNext(out.path)
	}

	err = r.q.FinishSeriesStep(ctx, models.FinishSeriesStepParams{
		WorkspaceID: r.e.WorkspaceID, ID: id, Status: status, PathID: optional(path), Outcome: optional(out.result),
		Detail: detail, Now: r.now,
	})

	return wrap("finish step", err)
}

/*
moveParams starts a move from where the journey stands in this job.
*/
func (r *run) moveParams() models.MoveSeriesEnrollmentParams {
	raw, _ := json.Marshal(r.variants)
	p := moveParams(r.e, r.now)
	p.Variants = raw
	p.Steps = int32Of(r.steps)
	r.steps = 0

	return p
}

/*
waitOf reads what the journey waits on.
*/
func (r *run) waitOf() wait {
	var w wait
	_ = json.Unmarshal(r.e.Wait, &w)

	return w
}

/*
withdraw keeps a journey that ended from sending what is still queued.
*/
func (s *Series) withdraw(ctx context.Context, q *models.Queries, e models.SeriesEnrollment) error {
	_, err := q.WithdrawSeriesRecipients(ctx, models.WithdrawSeriesRecipientsParams{
		WorkspaceID: e.WorkspaceID, SeriesEnrollmentID: &e.ID, Now: s.Clock.Now(),
	})

	return wrap("withdraw sends", err)
}
