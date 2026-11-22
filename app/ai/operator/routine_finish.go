/*
|--------------------------------------------------------------------------
| Ending A Routine's Run
|--------------------------------------------------------------------------
|
| A run ends found something, found nothing, failed or skipped, and the
| routine moves on with it: failures count toward a pause, a success
| resets them, a one-off routine is done once it ran, and a skip that
| means the owner lost what the routine needs pauses it. A dry run
| leaves the routine as it was.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
skipRoutine records a run that did not happen, pausing the routine when
the reason calls for it.
*/
func (e *Engine) skipRoutine(ctx context.Context, s *session, why skip) error {
	r := s.routine
	now := e.Clock.Now()
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		r.run.Status, r.run.Outcome, r.run.SkipReason, r.run.CompletedAt = "skipped", new(RunSkipped),
			&why.reason, &now
		if err := e.saveRun(ctx, q, r.run); err != nil || r.dryRun {
			return err
		}
		life := lifecycleOf(r.row)
		life.lastRun, life.outcome = &now, new(RunSkipped)
		if why.pause != "" {
			life.status, life.pause, life.next = "paused", &why.pause, nil
		}
		return e.saveLifecycle(ctx, q, r.row, life)
	})
	if err != nil {
		return err
	}
	return errRoutineDone
}

/*
finishRoutine stores the answer, ties its suggestions to it, records
the cost, and records how the run went on the run and the routine.
*/
func (e *Engine) finishRoutine(ctx context.Context, s *session, text, status string) error {
	r := s.routine
	outcome := outcomeOf(s, status)
	var answered models.OperatorMessage
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if s.run.ID != (ids.UUID{}) {
			var err error
			if answered, err = e.closeTurn(ctx, tx, s, ended{text: text, status: status}); err != nil {
				return err
			}
		}
		now := e.Clock.Now()
		e.endRun(s, status, outcome, now)
		if err := e.saveRun(ctx, q, r.run); err != nil || r.dryRun {
			return err
		}
		return e.saveLifecycle(ctx, q, r.row, e.afterRun(r, outcome, now))
	})
	if err != nil {
		return fmt.Errorf("finish routine run: %w", err)
	}
	if answered.ID != (ids.UUID{}) {
		e.publishAnswered(ctx, s, answered)
	}
	return nil
}

/*
outcomeOf is how a run ended: failed when the model did not finish,
findings when it suggested changes, else noop. Models often forget
mark_noop, so a run is quiet whenever it suggested nothing; a routine
that reports every time notifies on every run instead.
*/
func outcomeOf(s *session, status string) string {
	switch {
	case status != "done":
		return RunFailed
	case len(s.proposed) > 0:
		return RunFindings
	}
	return RunNoop
}

/*
ended is how the model's work ended: what it wrote and its status.
*/
type ended struct {
	text, status string
}

/*
closeTurn stores the answer, ties the suggestions to it, closes the AI
run and adds its cost to the month.
*/
func (e *Engine) closeTurn(ctx context.Context, tx *database.Tx, s *session, end ended,
) (models.OperatorMessage, error) {
	text, status := end.text, end.status
	if text == "" {
		text = map[string]string{"stopped": stoppedText, "failed": failedText}[status]
	}
	q := models.New(tx)
	msg, err := e.store(ctx, q, s, said{
		role: roleAssistant, body: text, status: status, steps: s.steps, run: &s.run.ID,
	})
	if err != nil {
		return msg, err
	}
	if err := e.attach(ctx, q, s, msg.ID); err != nil {
		return msg, err
	}
	if err := e.finishRun(ctx, q, s, status); err != nil {
		return msg, err
	}
	return msg, addSpend(ctx, tx, e, s)
}

/*
endRun fills the run row with how it ended.
*/
func (e *Engine) endRun(s *session, status, outcome string, now time.Time) {
	run := &s.routine.run
	run.Status, run.Outcome, run.CompletedAt = "completed", &outcome, &now
	run.CostMicros = s.tally.cost
	run.ChangesetsCount = 0
	for range s.proposed {
		run.ChangesetsCount++
	}
	switch outcome {
	case RunFailed:
		run.Status, run.ErrorCode = "failed", new(status)
		if s.failure != nil {
			run.ErrorMessage = new(truncate(s.failure.Error(), 1000))
		}
	case RunNoop:
		run.NoopReason = nilable(truncate(s.routine.noopReason, 500))
	}
}

/*
afterRun is the routine's lifecycle after a run ended: failures count
toward a pause, a success resets them, and a one-off routine is done
once its scheduled run ran.
*/
func (e *Engine) afterRun(r *routineRun, outcome string, now time.Time) lifecycle {
	life := lifecycleOf(r.row)
	life.lastRun, life.outcome = &now, &outcome
	if outcome == RunFailed {
		life.failures++
		if life.failures >= r.row.MaxConsecutiveFailures {
			reason := PauseFailures
			life.status, life.pause, life.next = "paused", &reason, nil
		}
	} else {
		life.failures, life.lastSuccess = 0, &now
	}
	if r.row.Cadence == "once" && r.run.Trigger == TriggerScheduled && life.status == "active" {
		life.status, life.next = "completed", nil
	}
	return life
}

/*
lifecycle is the part of a routine a run moves on.
*/
type lifecycle struct {
	next, lastRun, lastSuccess *time.Time
	status                     string
	pause, outcome             *string
	failures                   int32
}

/*
lifecycleOf reads a routine's lifecycle as it stands.
*/
func lifecycleOf(r models.OperatorRoutine) lifecycle {
	return lifecycle{
		next: r.NextRunAt, lastRun: r.LastRunAt, lastSuccess: r.LastSuccessAt, status: r.Status,
		pause: r.PauseReason, outcome: r.LastOutcome, failures: r.ConsecutiveFailures,
	}
}

/*
saveLifecycle writes a routine's lifecycle.
*/
func (e *Engine) saveLifecycle(ctx context.Context, q *models.Queries, r models.OperatorRoutine, l lifecycle) error {
	err := q.SaveOperatorRoutineLifecycle(ctx, models.SaveOperatorRoutineLifecycleParams{
		WorkspaceID: r.WorkspaceID, ID: r.ID, NextRunAt: l.next, Status: l.status, PauseReason: l.pause,
		ConsecutiveFailures: l.failures, LastRunAt: l.lastRun, LastSuccessAt: l.lastSuccess, LastOutcome: l.outcome,
		Now: e.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("save routine lifecycle: %w", err)
	}
	return nil
}

/*
saveRun writes a routine run row.
*/
func (e *Engine) saveRun(ctx context.Context, q *models.Queries, run models.OperatorRoutineRun) error {
	_, err := q.SaveOperatorRoutineRun(ctx, models.SaveOperatorRoutineRunParams{
		WorkspaceID: run.WorkspaceID, ID: run.ID, Status: run.Status, Outcome: run.Outcome,
		NoopReason: run.NoopReason, SkipReason: run.SkipReason, ErrorCode: run.ErrorCode,
		ErrorMessage: run.ErrorMessage, ThreadID: run.ThreadID, AiRunID: run.AiRunID,
		ChangesetsCount: run.ChangesetsCount, CostMicros: run.CostMicros, Notified: run.Notified,
		DeliveryResults: orArray(run.DeliveryResults), StartedAt: run.StartedAt, CompletedAt: run.CompletedAt,
		Now: e.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("save routine run: %w", err)
	}
	return nil
}

/*
orArray is a JSON array, empty when there is none.
*/
func orArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}
