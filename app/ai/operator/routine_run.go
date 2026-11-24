/*
|--------------------------------------------------------------------------
| Running A Routine
|--------------------------------------------------------------------------
|
| A run happens in three moves, like any Operator turn. A short
| transaction checks the run may happen: Operator is on, the owner still
| holds everything the routine needs, there is a model and the month's
| budgets are not spent. It then opens the run's thread with the
| rendered prompt and its AI run. The model works outside any
| transaction, for at most a few minutes, and a last transaction stores
| what it found.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
errRoutineDone marks a run that needs nothing more: it was skipped, or
another attempt already ran it.
*/
var errRoutineDone = errors.New("operator: routine run done")

/*
RunRoutine runs one queued run of a routine. A run that cannot happen
now is recorded as skipped; a failure of the model is recorded as a
failed run. Only a failure to reach the database returns an error, so
the job retries.
*/
func (e *Engine) RunRoutine(ctx context.Context, workspaceID, runID ids.UUID) error {
	e.prepare()
	s, err := e.openRoutine(ctx, workspaceID, runID)
	if errors.Is(err, errRoutineDone) {
		return nil
	}
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, routineTimeout)
	text, status := e.converse(runCtx, s)
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		status = "failed"
	}
	cancel()
	return e.finishRoutine(context.WithoutCancel(ctx), s, text, status)
}

/*
runRef names a run in its workspace.
*/
type runRef struct {
	workspace, run ids.UUID
}

/*
skip is why a run did not happen, and whether the routine pauses for it.
*/
type skip struct {
	reason string
	pause  string
}

/*
openRoutine checks the run may happen, then opens its thread and its AI
run. A run that may not happen is recorded as skipped.
*/
func (e *Engine) openRoutine(ctx context.Context, workspaceID, runID ids.UUID) (*session, error) {
	s := &session{}
	var skipped *skip
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		skipped, err = e.prepareRoutine(ctx, q, s, runRef{workspace: workspaceID, run: runID})
		if err != nil || skipped != nil {
			return err
		}
		return e.startRoutine(ctx, q, s)
	})
	if err != nil {
		return nil, err
	}
	if skipped != nil {
		return nil, e.skipRoutine(ctx, s, *skipped)
	}
	e.publishRun(ctx, s.routine.run)
	if s.llm, err = e.LLM.ForKey(ctx, s.key); err != nil {
		s.failure = err
		return nil, e.finishRoutine(context.WithoutCancel(ctx), s, "", "failed")
	}
	e.offerActions(ctx, s)
	return s, nil
}

/*
prepareRoutine locks the run and reads what it needs, saying why it may
not run when it may not.
*/
func (e *Engine) prepareRoutine(ctx context.Context, q *models.Queries, s *session, ref runRef) (*skip, error) {
	if err := e.lockRun(ctx, q, s, ref); err != nil {
		return nil, err
	}
	row, run := s.routine.row, s.routine.run
	if row.DeletedAt != nil || (run.Trigger == TriggerScheduled && row.Status != "active") {
		return &skip{reason: "routine_inactive"}, nil
	}
	var err error
	if s.settings, err = LoadSettings(ctx, q, ref.workspace); err != nil {
		return nil, err
	}
	if !s.settings.Enabled {
		return &skip{reason: "operator_disabled"}, nil
	}
	if reason, err := e.routineMember(ctx, q, s); err != nil || reason != nil {
		return reason, err
	}
	switch err := e.loadModel(ctx, q, s); {
	case errors.Is(err, ErrNoModel):
		return &skip{reason: "operator_no_model"}, nil
	case errors.Is(err, ErrCapped):
		return &skip{reason: "workspace_spend_capped"}, nil
	case err != nil:
		return nil, err
	}
	if over, err := e.overBudget(ctx, q, row); err != nil || over {
		return &skip{reason: "routine_budget_cap"}, err
	}
	e.applyRoutineSettings(s)
	return nil, nil
}

/*
lockRun locks a queued run and reads its routine; a run another attempt
already took is done.
*/
func (e *Engine) lockRun(ctx context.Context, q *models.Queries, s *session, ref runRef) error {
	run, err := q.LockOperatorRoutineRun(ctx, models.LockOperatorRoutineRunParams{
		WorkspaceID: ref.workspace, ID: ref.run,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && run.Status != "queued") {
		return errRoutineDone
	}
	if err != nil {
		return fmt.Errorf("lock routine run: %w", err)
	}
	row, err := q.FindAnyOperatorRoutine(ctx, models.FindAnyOperatorRoutineParams{
		WorkspaceID: ref.workspace, ID: run.RoutineID,
	})
	if err != nil {
		return fmt.Errorf("find routine: %w", err)
	}
	s.routine = &routineRun{
		row: row, run: run, scope: ScopeOf(row.Scope), dryRun: run.Trigger == TriggerDryRun,
	}
	s.m.Workspace.ID = ref.workspace
	return nil
}

/*
routineMember sets the narrowed membership the run reads with, pausing
the routine when its owner may no longer run it.
*/
func (e *Engine) routineMember(ctx context.Context, q *models.Queries, s *session) (*skip, error) {
	row := s.routine.row
	if row.OwnerMemberID == nil {
		return &skip{reason: PauseOwnerLost, pause: PauseOwnerLost}, nil
	}
	owner, ok, err := LoadMember(ctx, q, row.WorkspaceID, *row.OwnerMemberID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &skip{reason: PauseOwnerLost, pause: PauseOwnerLost}, nil
	}
	narrowed, ok, err := Narrow(ctx, q, owner, row)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &skip{reason: PauseOwnerLost, pause: PauseOwnerLost}, nil
	}
	s.m = narrowed
	return nil, nil
}

/*
overBudget reports whether the routine spent its own monthly budget.
*/
func (e *Engine) overBudget(ctx context.Context, q *models.Queries, r models.OperatorRoutine) (bool, error) {
	if r.MonthlyBudgetMicros == nil {
		return false, nil
	}
	now := e.Clock.Now().UTC()
	spent, err := q.SumOperatorRoutineCost(ctx, models.SumOperatorRoutineCostParams{
		WorkspaceID: r.WorkspaceID, RoutineID: r.ID,
		Since: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		return false, fmt.Errorf("sum routine cost: %w", err)
	}
	return spent >= *r.MonthlyBudgetMicros, nil
}

/*
applyRoutineSettings lays the routine's model, effort and write policies
over the workspace's: a policy is the stricter of the two, and nothing
ever applies on its own. A routine is offered its few tools directly
and reads full ids, which its report quotes to the team.
*/
func (e *Engine) applyRoutineSettings(s *session) {
	row := s.routine.row
	if row.Model != nil && *row.Model != "" && Offers(s.key, *row.Model) {
		s.model = *row.Model
	}
	if row.Effort != nil {
		s.settings.Effort = contracts.Effort(*row.Effort)
	}
	var own map[Group]Policy
	_ = json.Unmarshal(row.WritePolicies, &own)
	policies := map[Group]Policy{}
	for _, g := range Groups() {
		p := s.settings.PolicyFor(g)
		if p == PolicyAuto {
			p = PolicyAsk
		}
		if own[g] == PolicyOff {
			p = PolicyOff
		}
		policies[g] = p
	}
	s.settings.Policies = policies
	s.settings.ToolLayer, s.settings.CompactResults = false, false
}

/*
startRoutine opens the run's thread with the routine's prompt as its
question, and its AI run, and marks the run as running.
*/
func (e *Engine) startRoutine(ctx context.Context, q *models.Queries, s *session) error {
	r := s.routine
	now := e.Clock.Now()
	question, err := e.spellOut(ctx, s.m.Workspace.ID, e.renderPrompt(s, now))
	if err != nil {
		return err
	}
	s.question = question
	title := truncate(r.row.Name+" · "+now.In(location(r.row.Timezone)).Format("2006-01-02"), 200)
	if r.dryRun {
		title = truncate("Dry run · "+title, 200)
	}
	thread, err := q.CreateOperatorRoutineThread(ctx, models.CreateOperatorRoutineThreadParams{
		WorkspaceID: r.row.WorkspaceID, RoutineID: &r.row.ID, RoutineRunID: &r.run.ID, Title: title, Now: now,
	})
	if err != nil {
		return fmt.Errorf("open routine thread: %w", err)
	}
	s.thread = thread
	if _, err := e.store(ctx, q, s, said{role: roleUser, body: s.question, status: "done"}); err != nil {
		return err
	}
	if s.run, err = e.startRun(ctx, q, s); err != nil {
		return err
	}
	r.run.ThreadID, r.run.AiRunID, r.run.StartedAt, r.run.Status = &thread.ID, &s.run.ID, &now, "running"
	return e.saveRun(ctx, q, r.run)
}

/*
window is what a run covers: from the last run that went well, or a
week back for a first run, until now.
*/
func (e *Engine) window(s *session) (time.Time, time.Time, *time.Time) {
	r := s.routine.row
	now := e.Clock.Now()
	if r.LastSuccessAt != nil {
		return *r.LastSuccessAt, now, r.LastSuccessAt
	}
	return now.Add(-firstWindow), now, nil
}

/*
runWindow tells the model what the run covers, so a routine's
instructions can say "since the last run" and mean it.
*/
func (e *Engine) runWindow(s *session) string {
	start, end, last := e.window(s)
	ran := prompts.Task(taskRoutineFirstRun, nil)
	if last != nil {
		ran = prompts.Task(taskRoutineLastRun, prompts.Vars{"at": last.UTC().Format(time.RFC3339)})
	}
	return prompts.Task(taskRoutineWindow, prompts.Vars{
		"start": start.UTC().Format(time.RFC3339), "end": end.UTC().Format(time.RFC3339),
		"start_date": start.UTC().Format(time.DateOnly),
		"end_date":   end.UTC().AddDate(0, 0, 1).Format(time.DateOnly), "last_run": ran,
	})
}

/*
renderPrompt fills the variables older routines wrote into their
instructions; new ones need none, since every run is told its window.
*/
func (e *Engine) renderPrompt(s *session, _ time.Time) string {
	r := s.routine.row
	start, end, at := e.window(s)
	last := "never"
	if at != nil {
		last = at.UTC().Format(time.RFC3339)
	}
	return strings.NewReplacer(
		"{{workspace_name}}", s.m.Workspace.Name, "{{routine_name}}", r.Name, "{{last_run_at}}", last,
		"{{window_start}}", start.UTC().Format(time.RFC3339), "{{window_end}}", end.UTC().Format(time.RFC3339),
		"{{window_start_date}}", start.UTC().Format(time.DateOnly),
		"{{window_end_date}}", end.UTC().AddDate(0, 0, 1).Format(time.DateOnly),
	).Replace(r.Prompt)
}

/*
location is a routine's time zone, UTC when it is unknown.
*/
func location(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
