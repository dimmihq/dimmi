/*
|--------------------------------------------------------------------------
| Running Another Workflow
|--------------------------------------------------------------------------
|
| A run workflow step starts the live version of a reusable workflow on
| the same conversation as a run of its own, and waits for it to end:
| the called run starts from the caller's variables, and the caller
| goes on with what the called run left in them. A called workflow that
| talks to the customer takes the conversation from its caller while it
| runs and gives it back when it ends. Calls nest five deep at most;
| publishing already refuses calls that lead back to the caller, and a
| workflow already running on the conversation is never started twice.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
MaxCallDepth is how deep workflows may run one another.
*/
const MaxCallDepth = 5

/*
waitCall is the wait of a run for the workflow it runs.
*/
const waitCall = "call"

/*
InputCallDone tells a caller the workflow it ran ended.
*/
const InputCallDone = "call_done"

/*
callData is what a called run keeps of its call: how deep it sits.
*/
type callData struct {
	Depth int `json:"call_depth"`
}

/*
runRunWorkflow starts the reusable workflow and waits for it.
*/
func runRunWorkflow(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	r := settings[workflowspec.RunWorkflowSettings](s)

	var data callData
	_ = json.Unmarshal(st.run.TriggerData, &data)

	if data.Depth+1 > MaxCallDepth {
		return calledNothing("depth_limit", r.WorkflowID), nil
	}

	c, ok, err := st.callee(ctx, r.WorkflowID)
	if err != nil || !ok {
		return calledNothing(outcomeMissingRef, r.WorkflowID), err
	}

	if c.kind == workflowspec.KindCustomerFacing && st.run.Kind != workflowspec.KindCustomerFacing {
		return calledNothing("customer_facing_from_background", r.WorkflowID), nil
	}

	child, started, err := st.startCall(ctx, s, c, data.Depth+1)
	if err != nil || !started {
		return calledNothing(SkipAlreadyRunning, r.WorkflowID), err
	}

	return outcome{
		outcome: "calling", acted: true, wait: &waitState{Kind: waitCall, Child: child.String()},
		detail: map[string]any{
			"workflow_id": r.WorkflowID, "child_run_id": models.PublicID(models.PrefixWorkflowRun, child),
		},
	}, nil
}

/*
calledNothing is a call that could not start: the run goes on.
*/
func calledNothing(reason, workflow string) outcome {
	out := skipped(reason)
	out.detail = map[string]any{"workflow_id": workflow, "reason": reason}

	return out
}

/*
callee returns the live reusable workflow with the public id.
*/
func (st *state) callee(ctx context.Context, public string) (candidate, bool, error) {
	id, ok := st.decode(models.PrefixWorkflow, public)
	if !ok || id == st.run.WorkflowID {
		return candidate{}, false, nil
	}

	found, err := st.e.candidates(ctx, st.q, st.run.WorkspaceID, workflowspec.TriggerReusable)
	if err != nil {
		return candidate{}, false, err
	}

	for _, c := range found {
		if c.workflowID == id {
			return c, true, nil
		}
	}

	return candidate{}, false, nil
}

/*
startCall creates the called run with the caller's variables, hands it
the conversation when it talks to the customer, and queues it.
*/
func (st *state) startCall(ctx context.Context, s workflowspec.Step, c candidate, depth int) (ids.UUID, bool, error) {
	now := st.now()
	vars, err := json.Marshal(st.vars)
	if err != nil {
		return ids.UUID{}, false, fmt.Errorf("encode call variables: %w", err)
	}

	data, _ := json.Marshal(callData{Depth: depth})
	cursor, stepID := c.index.First(), s.ID

	run, err := st.q.CreateWorkflowRun(ctx, models.CreateWorkflowRunParams{
		ID: ids.NewV7At(now), WorkspaceID: st.run.WorkspaceID, WorkflowID: c.workflowID, VersionID: c.versionID,
		ConversationID: st.run.ConversationID, ContactID: st.run.ContactID, Kind: c.kind, Mode: "live",
		Trigger: workflowspec.TriggerReusable, TriggerData: data, ParentRunID: &st.run.ID, ParentStepID: &stepID,
		Chain: st.run.Chain, CursorStepID: &cursor, Vars: vars, Now: now,
	})
	if errors.Is(err, database.ErrNotFound) {
		return ids.UUID{}, false, nil
	}

	if err != nil {
		return ids.UUID{}, false, fmt.Errorf("start called run: %w", err)
	}

	if c.kind == workflowspec.KindCustomerFacing {
		if _, err := st.q.PassConversationClaim(ctx, models.PassConversationClaimParams{
			ToRunID: &run.ID, Now: now, WorkspaceID: st.run.WorkspaceID, ConversationID: st.conv.ID,
			FromRunID: &st.run.ID,
		}); err != nil {
			return ids.UUID{}, false, fmt.Errorf("pass conversation to called run: %w", err)
		}
	}

	advance := jobs.AdvanceWorkflowRunsArgs{WorkspaceID: run.WorkspaceID, RunIDs: []ids.UUID{run.ID}}

	return run.ID, true, st.push(ctx, advance)
}

/*
resumeRunWorkflow goes on once the called run ended, with the
variables it left.
*/
func resumeRunWorkflow(ctx context.Context, st *state, s workflowspec.Step, _ Input) (outcome, error) {
	child, err := ids.Parse(st.wait.Child)
	if err != nil {
		return outcome{outcome: "completed"}, nil //nolint:nilerr // a wait without its run just goes on
	}

	run, err := st.q.FindWorkflowRun(ctx, models.FindWorkflowRunParams{WorkspaceID: st.run.WorkspaceID, ID: child})
	if err != nil {
		return outcome{}, fmt.Errorf("find called run: %w", err)
	}

	left := map[string]any{}
	_ = json.Unmarshal(run.Vars, &left)
	maps.Copy(st.vars, left)
	st.subj = nil

	return outcome{outcome: "completed", acted: run.Acted, detail: map[string]any{
		"workflow_id":  settings[workflowspec.RunWorkflowSettings](s).WorkflowID,
		"child_run_id": models.PublicID(models.PrefixWorkflowRun, run.ID), "child_status": run.Status,
	}}, nil
}

/*
returnToCaller gives the conversation back to the run that called this
one and tells it the call ended.
*/
func (st *state) returnToCaller(ctx context.Context) error {
	parent := *st.run.ParentRunID
	if st.run.Kind == workflowspec.KindCustomerFacing {
		if _, err := st.q.PassConversationClaim(ctx, models.PassConversationClaimParams{
			ToRunID: &parent, Now: st.now(), WorkspaceID: st.run.WorkspaceID, ConversationID: st.conv.ID,
			FromRunID: &st.run.ID,
		}); err != nil {
			return fmt.Errorf("give conversation back to caller: %w", err)
		}
	}

	caller, err := st.q.FindWorkflowRun(ctx, models.FindWorkflowRunParams{WorkspaceID: st.run.WorkspaceID, ID: parent})
	if err != nil {
		return fmt.Errorf("find caller: %w", err)
	}

	if caller.Status != StatusWaiting {
		return nil
	}

	return st.push(ctx, jobs.AdvanceWorkflowRunArgs{
		WorkspaceID: caller.WorkspaceID, RunID: caller.ID, Seq: caller.Seq, Input: InputCallDone,
	})
}
