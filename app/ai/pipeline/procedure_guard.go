/*
|--------------------------------------------------------------------------
| Procedure Guardrails
|--------------------------------------------------------------------------
|
| What code, not the model, enforces while a procedure is followed: an
| action whose prerequisites have not run is refused before it runs; a
| reply that skips a required action is sent back with a note naming it,
| up to the step's retries, then the conversation goes to the team; a
| procedure whose required actions all ran is done; and a handover made
| while following it goes to the team and at the priority it names.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
gateProcedure refuses an action whose prerequisites have not run,
returning the refusal the model reads.
*/
func (r *Runner) gateProcedure(st *run, name string, t tool) (string, bool) {
	if !t.action || !st.proc.active() {
		return "", false
	}
	public := ""
	for _, live := range st.actions.offered {
		if live.Spec.Name == name {
			public = models.PublicID(models.PrefixAiAction, live.ActionID)
		}
	}
	plan, in := r.procedurePlan(st, succeededOnly)
	waits := plan.Prerequisites(public, in.Called, in.Tools)
	if len(waits) == 0 {
		return "", false
	}
	first := in.Tools[waits[0]]
	step := ""
	for _, a := range plan.Actions {
		if a.ActionID == waits[0] {
			step = " (Step " + a.Label + ")"
			break
		}
	}
	st.proc.guard(guardEvent{Type: "prerequisite_blocked", AttemptedTool: name, MissingTool: first})
	st.flags = appendOnce(st.flags, "procedure_blocked")
	out, _ := json.Marshal(map[string]string{
		"error":   "procedure_prerequisite_unmet",
		"message": "Cannot call " + name + " yet. You must call " + first + " first" + step + ".",
	})
	return string(out), true
}

/*
recordProcedureCall keeps what an action call did for the procedure:
that it was tried, called when it succeeded or now waits on the
customer or a teammate, and its result for later steps and rules.
*/
func (r *Runner) recordProcedureCall(st *run, live actionkit.Live, reply actionkit.CallReply, result json.RawMessage) {
	p := st.proc
	if p == nil {
		return
	}
	public := models.PublicID(models.PrefixAiAction, live.ActionID)
	if !slices.Contains(p.attempted, public) {
		p.attempted = append(p.attempted, public)
	}
	if reply.Call != nil {
		p.callIDs[public] = models.PublicID(models.PrefixAiActionCall, reply.Call.ID)
	}
	if (reply.Succeeded || reply.Awaiting || reply.Pending) && !slices.Contains(p.called, public) {
		p.called = append(p.called, public)
	}
	if !reply.Succeeded || len(result) == 0 {
		return
	}
	var decoded any
	if json.Unmarshal(result, &decoded) != nil {
		return
	}
	masked := r.maskValue(st.env, decoded)
	if out, err := json.Marshal(masked); err == nil && len(out) <= procedureOutputChars {
		p.outputs[live.Spec.Name] = masked
	}
}

/*
maskValue redacts personal data from every text inside a decoded
result.
*/
func (r *Runner) maskValue(e *env, v any) any {
	switch x := v.(type) {
	case string:
		return r.redactFor(e, x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = r.maskValue(e, item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = r.maskValue(e, item)
		}
		return out
	}
	return v
}

/*
verifyProcedureCompliance sends back a reply that skipped a required
action of the procedure, with a note naming it, and returns the reply
written again. After the step's retries it hands the conversation to
the team, or only notes it when the step says so.
*/
func (r *Runner) verifyProcedureCompliance(ctx context.Context, st *run, draft string) (string, error) {
	for {
		p := st.proc
		if !p.active() || st.handoff != nil || st.actions.waiting || st.actions.closed {
			return draft, nil
		}
		plan, in := r.procedurePlan(st, triedToo)
		missing := plan.Missing(in.Called, in.Tools)
		if len(missing) == 0 {
			return draft, nil
		}
		a := missing[0]
		tool := in.Tools[a.ActionID]
		if r.asksForInput(st, a.ActionID, draft) {
			p.guard(guardEvent{Type: "awaiting_input", MissingTool: tool, StepID: a.StepID})
			return draft, nil
		}
		if p.retries >= a.MaxRetries {
			return r.failProcedure(st, a, tool, draft), nil
		}
		p.retries++
		p.guard(guardEvent{Type: "compliance_retry", Attempt: p.retries, MissingTool: tool, StepID: a.StepID})
		st.flags = appendOnce(st.flags, "procedure_retry")
		st.loop.messages = append(st.loop.messages,
			contracts.ChatMessage{Role: contracts.ChatRoleAssistant, Parts: textParts(draft)},
			userMessage(retryNote(p.retries, a, tool, r.requiredInputs(st, a.ActionID))))
		st.loop.limit = procedureRetryCalls
		next, err := r.agentLoop(ctx, st)
		st.loop.limit = 0
		if err != nil {
			return draft, err
		}
		draft = next
	}
}

/*
failProcedure ends a procedure whose required action was still skipped
after every retry: it hands over, naming the step and the action, or
only records it when the step says to warn.
*/
func (r *Runner) failProcedure(st *run, a procedure.Planned, tool, draft string) string {
	p := st.proc
	if a.OnViolation == procedure.ViolateWarn {
		p.guard(guardEvent{Type: "compliance_warned", MissingTool: tool, StepID: a.StepID})
		return draft
	}
	p.guard(guardEvent{Type: "compliance_failed_handoff", MissingTool: tool, StepID: a.StepID})
	st.flags = appendOnce(st.flags, "procedure_guard")
	st.handoff = &handoffRequest{reason: "procedure_guard", note: "Procedure \"" + p.live.row.Name +
		"\": step " + a.Label + " requires " + tool + ", which dimmi did not call after " +
		strconv.Itoa(p.retries) + " reminders. Handed over so a teammate can finish it."}
	p.status = procedureHandedOff
	return ""
}

/*
retryNote is the note that sends a reply back, firmer each time.
*/
func retryNote(attempt int, a procedure.Planned, tool string, inputs []string) string {
	step := "Step " + a.Label
	switch attempt {
	case 1:
		return "<system_note>Reply not sent. The active procedure requires calling " + tool + " (" + step +
			") before answering. Call " + tool + " now (or ask the customer if you need a required parameter, " +
			"or call handoff if you cannot proceed).</system_note>"
	case 2:
		params := "no parameters you have to ask for"
		if len(inputs) > 0 {
			params = "these required parameters: " + strings.Join(inputs, ", ")
		}
		return "<system_note>Reply still not sent. " + step + " of the procedure requires the tool " + tool +
			", which takes " + params + ". Call " + tool + " now. If a required parameter is missing, ask the " +
			"customer for it in your reply instead.</system_note>"
	}
	return "<system_note>Last reminder: call " + tool + " (" + step + ") now. If you do not call it, or ask the " +
		"customer for what it needs, the conversation is handed to a teammate automatically.</system_note>"
}

/*
requiredInputs lists the arguments the model must supply to an action.
*/
func (r *Runner) requiredInputs(st *run, public string) []string {
	for _, live := range st.actions.offered {
		if models.PublicID(models.PrefixAiAction, live.ActionID) != public {
			continue
		}
		var schema struct {
			Required []string `json:"required"`
		}
		_ = json.Unmarshal(live.Spec.ModelSchema(), &schema)
		return schema.Required
	}
	return nil
}

/*
asksForInput reports whether the reply asks the customer for what a
required action needs: the action takes arguments only the customer can
give, and the reply asks a question.
*/
func (r *Runner) asksForInput(st *run, public, draft string) bool {
	return len(r.requiredInputs(st, public)) > 0 && strings.Contains(draft, "?")
}

/*
settleProcedure completes a procedure once every required action on its
path ran and no branch waits, so the next message starts fresh even
when the last action closed the turn's tools.
*/
func (r *Runner) settleProcedure(st *run) {
	if !st.proc.active() || st.handoff != nil || st.actions.waiting {
		return
	}
	plan, in := r.procedurePlan(st, triedToo)
	enforced := 0
	for _, a := range plan.Actions {
		if !a.Enforced() {
			continue
		}
		enforced++
		if !in.Called[a.ActionID] {
			return
		}
	}
	for _, n := range plan.Nodes {
		if n.Status == procedure.StatusLocked {
			return
		}
	}
	if enforced > 0 {
		st.proc.status, st.proc.reason = procedureCompleted, "every required action ran"
	}
}

/*
routeProcedureHandoff routes a handover made while following a
procedure: to the team and priority of the handoff chip on its path,
or else of its handover rule.
*/
func (r *Runner) routeProcedureHandoff(st *run) {
	p := st.proc
	if p == nil || st.handoff == nil {
		return
	}
	team, priority := "", ""
	if rule := p.live.def.HandoverRule; rule != nil {
		team, priority = rule.TeamID, rule.Priority
	}
	plan, _ := r.procedurePlan(st, triedToo)
	for _, n := range plan.Nodes {
		for _, b := range n.Step.Bindings {
			if b.Kind == procedure.BindHandoff && n.Status != procedure.StatusDone {
				team, priority = b.Ref, cmpOr(b.Priority, priority)
			}
		}
	}
	if id, ok := decodePublic(models.PrefixTeam, team); ok && slices.ContainsFunc(st.env.Triage.workspaceTeams,
		func(t models.Team) bool { return t.ID == id }) {
		st.changes.team = &id
	}
	if priority == "high" || priority == "urgent" {
		st.changes.priority = new(true)
	}
	if p.status == procedureActive {
		p.status = procedureHandedOff
	}
}
