/*
|--------------------------------------------------------------------------
| Procedure State And Trace
|--------------------------------------------------------------------------
|
| What a conversation's procedure keeps between turns, written when the
| turn commits, and what each run did with it, step by step, kept on the
| run. The state is read back when the next run opens, unless the
| procedure ended meanwhile: no longer live, the conversation closed or
| handed over, or a teammate replied.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
procedureTrace is what the run did with its procedure, step by step.
*/
func (r *Runner) procedureTrace(st *run) json.RawMessage {
	p := st.proc
	if p == nil {
		return nil
	}
	plan, in := r.procedurePlan(st, triedToo)
	steps := make([]map[string]any, 0, len(plan.Nodes))
	for _, n := range plan.Nodes {
		steps = append(steps, traceStep(n, in, p))
	}
	guards := p.guards
	if guards == nil {
		guards = []guardEvent{}
	}
	out, _ := json.Marshal(map[string]any{
		"procedure_id": p.live.publicID(), "procedure_name": p.live.row.Name, "version": p.live.version,
		"fit_score": p.fit, "status": p.status, "retries": p.retries, "steps": steps, "guard_events": guards,
		"reason": p.reason,
	})
	return out
}

/*
traceStep is one step of the trace: its status, the side code decided
or the conversation settled, or what it waits for, and the actions it
called.
*/
func traceStep(n procedure.Node, in procedure.Input, p *procRun) map[string]any {
	step := map[string]any{"step_id": n.ID, "kind": n.Step.Kind, "status": n.Status}
	if n.Deterministic {
		step["deterministic"] = true
		if n.Status == procedure.StatusLocked {
			step["needs"] = n.Needs
		} else {
			step["branch_taken"] = n.Branch
		}
	}
	if side := p.sides[n.ID]; side != "" {
		step["branch_taken"] = side
	}
	var called []string
	for _, b := range n.Step.Actions() {
		if in.Called[b.Ref] {
			called = append(called, b.Ref)
		}
		if call := p.callIDs[b.Ref]; call != "" {
			step["action_id"], step["action_call_id"] = b.Ref, call
		}
	}
	if len(called) > 0 {
		step["bindings_called"] = called
	}
	return step
}

/*
saveProcedureState keeps how far the conversation got in its procedure,
and ends any it moved away from. A handover ends it.
*/
func (r *Runner) saveProcedureState(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	if st.args.Kind != "" && st.args.Kind != "conversation" {
		return nil
	}
	q, now, e := models.New(tx), r.Clock.Now(), st.env
	for _, ended := range st.procEnded {
		if ended.stateID == nil {
			continue
		}
		if err := r.writeProcedureState(ctx, q, st, ended); err != nil {
			return err
		}
	}
	p := st.proc
	if p == nil {
		return nil
	}
	if d.outcome == outcomeHandoff || st.handoff != nil {
		p.status = procedureHandedOff
	}
	if p.stateID == nil {
		if err := q.EndProcedureStates(ctx, models.EndProcedureStatesParams{
			WorkspaceID: e.Workspace.ID, ConversationID: e.Conversation.ID, Status: procedureExited, Now: now,
		}); err != nil {
			return fmt.Errorf("end procedure states: %w", err)
		}
	}
	return r.writeProcedureState(ctx, q, st, p)
}

/*
writeProcedureState writes one procedure's state row.
*/
func (r *Runner) writeProcedureState(ctx context.Context, q *models.Queries, st *run, p *procRun) error {
	e, now := st.env, r.Clock.Now()
	plan, _ := r.procedurePlan(&run{env: e, proc: p, actions: st.actions}, triedToo)
	completed := []string{}
	for _, n := range plan.Nodes {
		if n.Status == procedure.StatusDone {
			completed = append(completed, n.ID)
		}
	}
	outputs, _ := json.Marshal(p.outputs)
	branches, _ := json.Marshal(cmpSides(p.sides))
	called := p.called
	if called == nil {
		called = []string{}
	}
	var ended *time.Time
	if p.status != procedureActive {
		ended = &now
	}
	runID := st.rec.Run.ID
	if p.stateID != nil {
		_, err := q.UpdateProcedureState(ctx, models.UpdateProcedureStateParams{
			Status: p.status, CompletedStepIds: completed, CalledActionIds: called, StepOutputs: outputs,
			Branches: branches,
			RunID:    &runID, Now: now, EndedAt: ended, WorkspaceID: e.Workspace.ID, ID: *p.stateID,
		})
		if err != nil {
			return fmt.Errorf("update procedure state: %w", err)
		}
		return nil
	}
	row, err := q.StartProcedureState(ctx, models.StartProcedureStateParams{
		WorkspaceID: e.Workspace.ID, ConversationID: e.Conversation.ID, ProcedureID: p.live.row.ID,
		ProcedureVersionID: p.live.row.LiveVersionID, Status: p.status, CompletedStepIds: completed,
		CalledActionIds: called, StepOutputs: outputs, Branches: branches, RunID: &runID, Now: now, EndedAt: ended,
	})
	if err != nil {
		return fmt.Errorf("start procedure state: %w", err)
	}
	p.stateID = &row.ID
	return nil
}

/*
saveProcedureTrace keeps the run's procedure trace on its row.
*/
func (r *Runner) saveProcedureTrace(ctx context.Context, tx *database.Tx, st *run) error {
	trace := r.procedureTrace(st)
	if trace == nil || st.rec == nil {
		return nil
	}
	return models.New(tx).SetRunProcedureTrace(ctx, models.SetRunProcedureTraceParams{
		ProcedureTrace: trace, WorkspaceID: st.rec.Run.WorkspaceID, ID: st.rec.Run.ID,
	})
}

/*
activeProcedure is the procedure a conversation was following when the
run opened.
*/
type activeProcedure struct {
	state models.ConversationProcedureState
	proc  liveProcedure
}

/*
loadActiveProcedure reads the procedure the conversation is following.
It ends it instead when the procedure is no longer live, the
conversation was closed or handed over, or a teammate replied since.
*/
func loadActiveProcedure(ctx context.Context, q *models.Queries, e *env) error {
	if e.Conversation.ID == (ids.UUID{}) {
		return nil
	}
	ws := e.Conversation.WorkspaceID
	state, err := q.FindActiveProcedureState(ctx, models.FindActiveProcedureStateParams{
		WorkspaceID: ws, ConversationID: e.Conversation.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find procedure state: %w", err)
	}
	end := ""
	i := slices.IndexFunc(e.AllProcedures, func(p liveProcedure) bool { return p.row.ID == state.ProcedureID })
	switch {
	case i < 0:
		end = procedureExited
	case e.Conversation.Status == "closed":
		end = procedureExited
	case e.Conversation.AiState == aiStateHandedOff:
		end = procedureHandedOff
	}
	if end == "" {
		replied, err := q.MemberRepliedSince(ctx, models.MemberRepliedSinceParams{
			WorkspaceID: ws, ConversationID: e.Conversation.ID, Since: state.UpdatedAt,
		})
		if err != nil {
			return fmt.Errorf("check teammate reply: %w", err)
		}
		if replied {
			end = procedureHandedOff
		}
	}
	if end != "" {
		return q.EndProcedureStates(ctx, models.EndProcedureStatesParams{
			WorkspaceID: ws, ConversationID: e.Conversation.ID, Status: end, Now: e.now,
		})
	}
	e.ActiveProcedure = &activeProcedure{state: state, proc: e.AllProcedures[i]}
	return nil
}

/*
cmpSides returns the sides taken, never nil, so they are stored as an
object.
*/
func cmpSides(sides map[string]string) map[string]string {
	if sides == nil {
		return map[string]string{}
	}
	return sides
}
