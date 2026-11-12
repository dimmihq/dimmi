/*
|--------------------------------------------------------------------------
| Run Lifecycle
|--------------------------------------------------------------------------
|
| A run ends completed when its path runs out. A customer-facing run
| that did nothing at all, only branches with empty paths, ends passed,
| so it never blocks the conversation from what comes next. Ending lets
| go of the conversation; one a customer-facing run routed to nobody
| goes to the inbox's own assignment, as if no workflow had held it.
|
| A run saves its progress once per job: cursor, wait, variables and the
| number of steps it executed, with its sequence moved on.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/assignment"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The wait kinds and what may interrupt a wait.
*/
const (
	waitInput         = "input"
	waitTimer         = "timer"
	interruptCustomer = "customer_message"
	interruptTeammate = "teammate_reply"
)

/*
waitState is what a waiting run waits for: its kind and step, when it
times out, what interrupts it, and for questions how many answers it
had and whether it already asked again; a run waiting for a workflow
it runs keeps that run's id in Child. A run waiting on the agent
also keeps when the agent took over, how long a silent customer is
waited for, and what the agent must honor meanwhile.
*/
type waitState struct {
	Kind      string                      `json:"kind,omitempty"`
	StepID    string                      `json:"step_id,omitempty"`
	Until     *time.Time                  `json:"until,omitempty"`
	Interrupt []string                    `json:"interrupt,omitempty"`
	Attempts  int                         `json:"attempts,omitempty"`
	Retried   bool                        `json:"retried,omitempty"`
	Since     *time.Time                  `json:"since,omitempty"`
	Quiet     string                      `json:"quiet,omitempty"`
	Agent     *workflowspec.AgentHandling `json:"dimmi,omitempty"`
	Child     string                      `json:"child,omitempty"`
}

/*
interrupts reports whether the wait ends early on the source.
*/
func (w waitState) interrupts(source string) bool {
	return w.Kind == waitTimer && slices.Contains(w.Interrupt, source)
}

/*
readWait decodes a stored wait.
*/
func readWait(raw json.RawMessage) waitState {
	var w waitState
	_ = json.Unmarshal(raw, &w)

	return w
}

/*
outcome is what a step did and where the run goes: on along its path,
into one of the step's paths, into a wait, or to its end.
*/
type outcome struct {
	path    string
	wait    *waitState
	end     string
	status  string
	outcome string
	detail  map[string]any
	acted   bool
}

/*
trace records the step's row, or closes the row of a step that waited.
*/
func (st *state) trace(ctx context.Context, step workflowspec.Step, out outcome) error {
	status, detail := traceStatus(out), traceDetail(out)
	if st.resuming {
		return st.finishTrace(ctx, step, out)
	}

	now := st.now()

	var finished *time.Time
	if status != "waiting" {
		finished = &now
	}

	_, err := st.q.CreateWorkflowRunStep(ctx, models.CreateWorkflowRunStepParams{
		ID: ids.NewV7At(now), WorkspaceID: st.run.WorkspaceID, RunID: st.run.ID, WorkflowID: st.run.WorkflowID,
		VersionID: st.run.VersionID, StepID: step.ID, Kind: step.Kind, Status: status, PathID: optional(out.path),
		Outcome: optional(out.outcome), Detail: detail, Now: now, FinishedAt: finished,
	})
	if err != nil {
		return fmt.Errorf("trace step: %w", err)
	}

	return nil
}

/*
finishTrace closes the row of a step that waited, and opens a new
waiting row when it waits again.
*/
func (st *state) finishTrace(ctx context.Context, step workflowspec.Step, out outcome) error {
	status := traceStatus(out)
	if status == "waiting" {
		return nil
	}

	err := st.q.FinishWorkflowRunStep(ctx, models.FinishWorkflowRunStepParams{
		Status: status, PathID: optional(out.path), Outcome: optional(out.outcome), Detail: traceDetail(out),
		Now:         new(st.now()),
		WorkspaceID: st.run.WorkspaceID, RunID: st.run.ID, StepID: step.ID,
	})
	if err != nil {
		return fmt.Errorf("finish step trace: %w", err)
	}

	return nil
}

/*
traceStatus is the trace status of an outcome.
*/
func traceStatus(out outcome) string {
	switch {
	case out.wait != nil:
		return "waiting"
	case out.status != "":
		return out.status
	}

	return "completed"
}

/*
traceDetail is the trace detail of an outcome, an empty object for none.
*/
func traceDetail(out outcome) json.RawMessage {
	detail, err := json.Marshal(out.detail)
	if err != nil || out.detail == nil {
		return json.RawMessage("{}")
	}

	return detail
}

/*
optional returns the text as a nullable column, nil when empty.
*/
func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

/*
finish ends the run for the reason.
*/
func (st *state) finish(ctx context.Context, reason string) {
	st.status, st.reason, st.cursor = statusFor(reason), reason, ""
	if st.status == StatusCompleted && st.run.Kind == workflowspec.KindCustomerFacing && !st.acted {
		st.status, st.reason = StatusPassed, EndNoAction
	}

	_ = ctx
}

/*
fail ends the run as failed with the reason and a message.
*/
func (st *state) fail(ctx context.Context, reason, message string) {
	st.finish(ctx, reason)
	st.status = StatusFailed
	st.failure, _ = json.Marshal(map[string]string{"code": reason, "message": message, "step_id": st.cursor})
}

/*
statusFor returns the status a run ends with for a reason.
*/
func statusFor(reason string) string {
	switch reason {
	case EndCompleted, EndInterrupted:
		return StatusCompleted
	case EndStepBudget, EndDefinitionError, EndInternalError:
		return StatusFailed
	case EndExpired:
		return StatusExpired
	}

	return StatusExited
}

/*
ended reports whether the run is over.
*/
func (st *state) ended() bool {
	return st.status != StatusRunning && st.status != StatusWaiting
}

/*
save writes the run's progress and queues what comes next: the rest of
its steps, its timer, or, once it ended, the conversation's release.
*/
func (st *state) save(ctx context.Context) error {
	params, err := st.saveParams()
	if err != nil {
		return err
	}

	saved, err := st.q.SaveWorkflowRun(ctx, params)
	if err != nil {
		return fmt.Errorf("save workflow run: %w", err)
	}

	switch {
	case st.ended():
		return st.release(ctx)
	case st.status == StatusRunning:
		return st.push(ctx, jobs.AdvanceWorkflowRunArgs{
			WorkspaceID: saved.WorkspaceID, RunID: saved.ID, Seq: saved.Seq,
		})
	case st.wait.Until != nil:
		return st.push(ctx, jobs.AdvanceWorkflowRunArgs{
			WorkspaceID: saved.WorkspaceID, RunID: saved.ID, Seq: saved.Seq, Wake: true,
		}, queue.At(*st.wait.Until))
	}

	return nil
}

/*
saveParams is the run's row as it now is.
*/
func (st *state) saveParams() (models.SaveWorkflowRunParams, error) {
	vars, err := json.Marshal(st.vars)
	if err != nil {
		return models.SaveWorkflowRunParams{}, fmt.Errorf("encode run variables: %w", err)
	}

	p := models.SaveWorkflowRunParams{
		Status: st.status, CursorStepID: optional(st.cursor), Vars: vars, Acted: st.acted,
		StepsExecuted: st.run.StepsExecuted + st.steps, Error: st.failure, Now: st.now(),
		WorkspaceID: st.run.WorkspaceID, ID: st.run.ID,
	}

	if st.status == StatusWaiting {
		p.Wait, _ = json.Marshal(st.wait)
		p.WakeAt = st.wait.Until
	}

	if st.ended() {
		now := st.now()
		p.EndedAt, p.EndReason = &now, optional(st.reason)
	}

	return p, nil
}

/*
push queues a job in the run's transaction.
*/
func (st *state) push(ctx context.Context, args queue.Args, opts ...queue.Opt) error {
	if err := st.e.Queue.PushTx(ctx, st.tx, args, opts...); err != nil {
		return fmt.Errorf("queue %s: %w", args.Kind(), err)
	}

	return nil
}

/*
release lets go of the conversation once a customer-facing run ended,
and hands an open conversation nobody holds to the inbox's assignment.
A run another workflow called hands back to its caller instead.
*/
func (st *state) release(ctx context.Context) error {
	if st.run.ParentRunID != nil {
		return st.returnToCaller(ctx)
	}

	if st.run.Kind != workflowspec.KindCustomerFacing {
		return nil
	}

	if err := st.q.ReleaseConversationClaim(ctx, models.ReleaseConversationClaimParams{
		Now: st.now(), WorkspaceID: st.run.WorkspaceID, ConversationID: st.conv.ID, RunID: &st.run.ID,
	}); err != nil {
		return fmt.Errorf("release conversation: %w", err)
	}

	open := st.conv.Status == string(enums.ConversationStatusOpen)
	if !open || st.conv.AssigneeMemberID != nil || st.status == StatusExited {
		return nil
	}

	return st.push(ctx, jobs.AssignConversationArgs{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID, Reason: assignment.ReasonArrival,
	})
}
