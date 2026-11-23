/*
|--------------------------------------------------------------------------
| Step Reports
|--------------------------------------------------------------------------
|
| What happens when dimmi says a step of the procedure it follows is
| finished, covered already, or that the whole procedure is. Code holds
| it to the steps as written: a step that asks waits for the customer, a
| step that needs their yes is asked first, and a procedure with a step
| left is not finished.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/procedure"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
finishProcedureStep marks a step finished. A step that waits on the
customer is finished only once they have given what it needs.
*/
func (r *Runner) finishProcedureStep(ctx context.Context, st *run, number string) (string, error) {
	return r.reportProcedureStep(ctx, st, number)
}

/*
skipProcedureStep marks a step the customer covered before it was
asked, held to the same test: what the step needs must be in their
messages.
*/
func (r *Runner) skipProcedureStep(ctx context.Context, st *run, number string) (string, error) {
	return r.reportProcedureStep(ctx, st, number)
}

/*
reportProcedureStep takes the agent's word that a step is finished or
already covered, and refuses it when the step waits on the customer and
they have not given what it needs. A step that takes an action is
finished by taking it, with one exception: the customer turned the
action down, which their own messages have to show.
*/
func (r *Runner) reportProcedureStep(ctx context.Context, st *run, number string) (string, error) {
	number = stepNumber(number)
	plan, _ := r.procedurePlan(st, triedToo)
	at := slices.IndexFunc(plan.Nodes, func(n procedure.Node) bool { return n.Label == number })
	if at < 0 {
		return "", errToolArguments
	}
	n := plan.Nodes[at]
	if len(n.Step.Actions()) > 0 {
		return r.reportActionStep(ctx, st, n)
	}
	if handsOverIn(n.Step) && st.handoff == nil {
		return st.proc.refused(number, "Step "+number+" is finished by handing the conversation over: say what it "+
			"tells the customer and call handoff now, in this reply. Do not go on to another step.")
	}
	if twoSided(n) && n.Status != procedure.StatusDone {
		if !r.decideSide(ctx, st, n) {
			return undecided(number)
		}
		return r.markProcedureStep(st, number)
	}
	if n.Status == procedure.StatusDone {
		return nothingChanged("Step " + number + " is already finished. There is no need to report it again. " +
			"Carry on with the active step.")
	}
	if st.proc.waitsOn(n.Step) && !r.customerGave(ctx, st, n) {
		if n.ID == st.proc.asked {
			return st.proc.refused(number, "Step "+number+" is not finished: the customer's messages do not give "+
				"everything it needs yet. Stay on this step. Thank them for what they gave, ask for what is "+
				"missing, and end your reply there.")
		}
		return st.proc.refused(number, "The customer has not given what step "+number+" needs yet. Do this step in "+
			"your reply now, ask what it asks, and end your reply there. Mark it done in a later reply, once "+
			"their messages give everything it needs, however many exchanges that takes. Do no later step "+
			"in this reply.")
	}
	return r.markProcedureStep(st, number)
}

/*
reportActionStep answers a report on a step that takes an action. Such
a step is finished by taking the action, so the report changes nothing,
unless the customer's own messages turn the action down: then the step
is done without it.
*/
func (r *Runner) reportActionStep(ctx context.Context, st *run, n procedure.Node) (string, error) {
	if n.Status != procedure.StatusDone && r.stepDeclined(ctx, st, n) {
		st.proc.guard(guardEvent{Type: "step_declined", StepID: n.ID})
		return r.markProcedureStep(st, n.Label)
	}
	return nothingChanged("A step that calls a tool is finished by calling the tool. There is no need to " +
		"report it. Carry on. Only when the customer's own messages turn down what the step does is it " +
		"reported, with step_skipped and their words.")
}

/*
customerGave reports whether the customer has given what a step needs,
when the agent says so: the model that read the conversation has to
find it likely too. When no model could
be asked, the step counts as answered only once the customer has
replied to it.
*/
func (r *Runner) customerGave(ctx context.Context, st *run, n procedure.Node) bool {
	if given, ok := r.stepGiven(ctx, st, n); ok {
		return given >= judged
	}
	return n.ID == st.proc.asked
}

/*
nothingChanged answers a report that changes nothing, with a note
saying why.
*/
func nothingChanged(note string) (string, error) {
	out, err := json.Marshal(map[string]any{"ok": true, "nothing_changed": true, "note": note})
	return string(out), err
}

/*
undecided tells dimmi that the side of a plain-words condition is not
settled yet, and what to do either way.
*/
func undecided(number string) (string, error) {
	out, err := json.Marshal(map[string]any{"ok": false, "active_step": number, "undecided": true, "note": "It " +
		"is not settled yet which way step " + number + " goes. If it depends on something only the customer " +
		"can say, ask them in your reply now and end your reply there. If it depends on what you already " +
		"know, go straight to the steps of the side that holds: there is no need to report this step."})
	return string(out), err
}

/*
stepRefused tells dimmi a step it reported is not finished, and what to
do about it.
*/
func stepRefused(number, note string) (string, error) {
	out, err := json.Marshal(map[string]any{"ok": false, "active_step": number, "note": note})
	return string(out), err
}

/*
refused counts a refused report of a step in this run and, from the
second, tells dimmi to stop reporting it and answer the customer: the
step will not be finished in this reply however often it is reported.
*/
func (p *procRun) refused(number, note string) (string, error) {
	if p.refusals == nil {
		p.refusals = map[string]int{}
	}
	p.refusals[number]++
	if p.refusals[number] > 1 {
		note = "You were already told step " + number + " is not finished. Do not report it again in this reply: " +
			"write your reply to the customer now, doing what the step asks."
	}

	return stepRefused(number, note)
}

/*
stepLeft is the number of the first step still to do that cannot be
done without the customer or a tool: one that asks, or one that takes an
action. A procedure with such a step left is not finished.
*/
func (r *Runner) stepLeft(st *run) string {
	plan, _ := r.procedurePlan(st, triedToo)
	for _, n := range plan.Nodes {
		open := n.Status == procedure.StatusActive || n.Status == procedure.StatusPending
		if open && (st.proc.waitsOn(n.Step) || len(n.Step.Actions()) > 0) {
			return n.Label
		}
	}
	return ""
}

/*
stepLeftNote tells dimmi the procedure it called finished still has a
step to do.
*/
func stepLeftNote(number string) (string, error) {
	out, err := json.Marshal(map[string]any{"ok": false, "active_step": number, "note": "The procedure is not " +
		"finished: step " + number + " is still to do. Waiting for the customer's answer does not finish " +
		"it; the procedure stays open until they reply. Report nothing now and end your reply on the step " +
		"you are on. If the customer has turned down what that step does, report it with step_skipped and " +
		"their words. Only when the customer no longer wants this at all, use exited_topic_changed."})
	return string(out), err
}

/*
stepUnfinished tells dimmi a step it tried to finish still lacks
something from the customer, so it stays on the step and asks for it.
*/
func stepUnfinished(number, missing string) (string, error) {
	out, err := json.Marshal(map[string]any{"ok": false, "active_step": stepNumber(number), "note": "Step " +
		stepNumber(number) + " is not finished: the customer has not given " + missing + ". Stay on this step. " +
		"Thank them for what they gave, ask for what is missing, and end your reply there."})
	return string(out), err
}

/*
markProcedureStep keeps the step with the number as finished and tells
dimmi which step is active next.
*/
func (r *Runner) markProcedureStep(st *run, number string) (string, error) {
	number = stepNumber(number)
	plan, _ := r.procedurePlan(st, triedToo)
	at := slices.IndexFunc(plan.Nodes, func(n procedure.Node) bool { return n.Label == number })
	if at < 0 {
		return "", errToolArguments
	}
	st.proc.done = appendOnce(st.proc.done, plan.Nodes[at].ID)
	answered := plan.Nodes[at].ID == st.proc.asked

	next := ""
	plan, _ = r.procedurePlan(st, triedToo)
	for _, n := range plan.Nodes {
		if n.Status == procedure.StatusActive {
			next = n.Label
		}
	}
	out, err := json.Marshal(map[string]any{"ok": true, "done": number, "active_step": next, "answered": answered})
	return string(out), err
}

/*
stepNumber reads a step's number as the agent wrote it.
*/
func stepNumber(number string) string {
	return strings.TrimSuffix(strings.TrimSpace(number), ".")
}

/*
movedOn reports whether the customer has left the procedure's request,
as the model that read the conversation judged it. Leaving a procedure
is the customer's call, never the agent's: an agent that finds a step
awkward is held to it. When no model can say, the agent's word stands.
*/
func (r *Runner) movedOn(ctx context.Context, st *run) bool {
	vars := prompts.Vars{"request": st.proc.live.def.Description, "conversation": r.conversationLines(st.env)}
	answers := r.judge(ctx, st, contracts.DecisionRequest{
		State:     prompts.Judgment("moved_on", vars).State,
		Questions: map[string]contracts.DecisionQuestion{"moved_on": question("moved_on", vars)},
	})
	moved, ok := answers["moved_on"]

	return !ok || moved >= judged
}

/*
stayNote tells dimmi it may not leave the procedure: the customer still
wants what it is for.
*/
func stayNote() (string, error) {
	out, err := json.Marshal(map[string]any{"ok": false, "note": "The customer still wants what this procedure " +
		"is for, so it is not left. Carry on with the active step. Steps marked Decided were settled from the " +
		"customer's data; follow the side they took."})
	return string(out), err
}

/*
handsOverIn reports whether a step hands the conversation to a team, by
a handoff chip in its words.
*/
func handsOverIn(s procedure.Step) bool {
	return slices.ContainsFunc(s.Bindings, func(b procedure.Binding) bool { return b.Kind == procedure.BindHandoff })
}
