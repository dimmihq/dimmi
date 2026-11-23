/*
|--------------------------------------------------------------------------
| Judging Steps
|--------------------------------------------------------------------------
|
| Two things about a step cannot be read off its structure: whether
| doing it needs something back from the customer, and whether the
| customer has given it. The agent's word is not enough for either, and
| the wording of a step is not a rule to match on. The decision model
| answers both.
|
| What a step needs is asked once for a step's words and remembered, so
| a procedure costs one call the first time it is followed, not one per
| conversation. Whether the customer has given it is asked each time the
| agent reports such a step finished: the model reads the conversation
| and the step, and the report is refused when something is missing.
|
| A judgment is never guessed from a step's wording. When the decision
| model fails it is asked again, and when it still fails, or none is set
| up, the workspace's own answer model is asked the same questions and
| must answer in a fixed shape. Only when no model answers at all does
| code fall back to what it knows for certain: a step with no action is
| taken to wait on the customer, and it counts as answered once they
| have replied to it.
|
| The questions are the step_*.md and condition_*.md files under
| resources/prompts/tasks/judgments.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/procedure"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
stepKey names a step by its words, so what is learned about it holds
for every procedure and version that words it the same.
*/
func stepKey(s procedure.Step) string {
	return "s_" + hashing.SHA256Hex([]byte(s.Text))[:16]
}

/*
waitsOn reports whether a step waits on the customer: it takes no
action and doing it needs something back from them, as the model judged
it. A step no model could judge is taken to wait, which at worst holds
the steps after it for one more reply.
*/
func (p *procRun) waitsOn(s procedure.Step) bool {
	if len(s.Actions()) > 0 {
		return false
	}
	if waits, ok := p.waits[stepKey(s)]; ok {
		return waits
	}
	return true
}

/*
learnSteps returns, for each step of the procedure that takes no
action, whether it waits on the customer. Steps judged before are read
from memory; the rest are asked of the decision model in one call.
*/
func (r *Runner) learnSteps(ctx context.Context, st *run) map[string]bool {
	plan, _ := r.procedurePlan(st, triedToo)
	waits := map[string]bool{}
	req := contracts.DecisionRequest{Questions: map[string]contracts.DecisionQuestion{}}
	var steps strings.Builder
	for _, n := range plan.Nodes {
		steps.WriteString("\nStep " + n.Label + ": " + n.Step.Text)
		if len(n.Step.Actions()) > 0 || strings.TrimSpace(n.Step.Text) == "" {
			continue
		}
		key := stepKey(n.Step)
		if known, ok := r.stepWaits.Load(key); ok {
			waits[key], _ = known.(bool)
			continue
		}
		req.Questions[key] = question("step_waits", prompts.Vars{"step": n.Label})
	}
	if len(req.Questions) == 0 {
		return waits
	}
	req.State = prompts.Judgment("step_waits", prompts.Vars{"steps": steps.String()}).State
	for key, needs := range r.judge(ctx, st, req) {
		waits[key] = needs >= judged
		r.stepWaits.Store(key, waits[key])
	}

	return waits
}

/*
advanceLimit bounds how many steps in a row one message can be found to
cover.
*/
const advanceLimit = 4

/*
advanceGiven moves the procedure past the steps the customer has
already answered, before the agent writes anything: while the active
step waits on the customer and the model that reads the conversation
finds everything it needs there, the step is marked done. Progress then
does not hang on the agent remembering to report it, and a customer who
said it all up front is not asked again.
*/
func (r *Runner) advanceGiven(ctx context.Context, st *run) {
	for range advanceLimit {
		plan, _ := r.procedurePlan(st, triedToo)
		at := slices.IndexFunc(plan.Nodes, func(n procedure.Node) bool { return n.Status == procedure.StatusActive })
		if at < 0 {
			return
		}
		n := plan.Nodes[at]
		if !r.advance(ctx, st, n) {
			return
		}
	}
}

/*
advance moves past one active step when the conversation already
settles it: a condition in plain words whose side can be told, or a
step that waits on the customer who has given what it needs. With
nobody else saying so, the model must be sure of it; when it only
leans that way the step stays, and the agent is told to check before
asking again. It reports whether it moved.
*/
func (r *Runner) advance(ctx context.Context, st *run, n procedure.Node) bool {
	if twoSided(n) {
		return r.decideSide(ctx, st, n)
	}
	if len(n.Step.Actions()) > 0 || handsOverIn(n.Step) || !st.proc.waitsOn(n.Step) {
		return false
	}
	given, ok := r.stepGiven(ctx, st, n)
	if ok && given >= judged && given < sure {
		st.proc.maybe = n.Label
	}
	if !ok || given < sure {
		return false
	}
	st.proc.done = appendOnce(st.proc.done, n.ID)
	st.proc.covered = append(st.proc.covered, n.Label)
	r.publishStep(ctx, st, PreviewStep{
		Tool: toolProcedureStatus, Summary: "Step " + n.Label + ": the customer already answered it", OK: true,
	})

	return true
}

/*
twoSided reports whether a node is a condition written in plain words
with steps on either side, which a model must decide.
*/
func twoSided(n procedure.Node) bool {
	return n.Step.Kind == procedure.KindCondition && n.Branch == "both"
}

/*
decideSide asks the model that reads the conversation which side of a
plain-words condition holds, and takes it when that can be told: the
side is kept with the procedure's state, so the steps of the other side
are off the path from here on. It reports whether a side was taken.
*/
func (r *Runner) decideSide(ctx context.Context, st *run, n procedure.Node) bool {
	known, _ := json.Marshal(st.proc.outputs)
	vars := prompts.Vars{
		"condition": n.Step.Text, "today": st.env.now.Format("Monday 2 January 2006"),
		"lookups": truncate(string(known), procedureChars), "conversation": r.conversationLines(st.env),
	}
	answers := r.judge(ctx, st, contracts.DecisionRequest{
		State: prompts.Judgment("condition_clear", vars).State,
		Questions: map[string]contracts.DecisionQuestion{
			"clear": question("condition_clear", vars),
			"holds": question("condition_holds", vars),
		},
	})
	told, ok := answers["clear"]
	if !ok || told < judged {
		return false
	}
	side, said := procedure.SideElse, "no"
	if answers["holds"] >= judged {
		side, said = procedure.SideThen, "yes"
	}
	st.proc.sides[n.ID] = side
	st.proc.done = appendOnce(st.proc.done, n.ID)
	r.publishStep(ctx, st, PreviewStep{
		Tool: toolProcedureStatus, Summary: "Decided step " + n.Label + ": " + said, OK: true,
	})

	return true
}

/*
stepDeclined reports whether the customer turned down what a step that
takes an action would do, as the decision model reads their messages.
With no decision model, or none that answers, it cannot be told and the
step stands.
*/
func (r *Runner) stepDeclined(ctx context.Context, st *run, n procedure.Node) bool {
	vars := prompts.Vars{"step": n.Step.Text, "conversation": r.conversationLines(st.env)}
	req := contracts.DecisionRequest{
		State:     prompts.Judgment("step_declined", vars).State,
		Questions: map[string]contracts.DecisionQuestion{"declined": question("step_declined", vars)},
	}
	declined, ok := r.judge(ctx, st, req)["declined"]

	return ok && declined >= judged
}

/*
stepGiven asks the decision model how likely it is that the customer
has given what a step needs from them. The second result is false when
it could not be asked.
*/
func (r *Runner) stepGiven(ctx context.Context, st *run, n procedure.Node) (float64, bool) {
	vars := prompts.Vars{"step": n.Step.Text, "conversation": r.conversationLines(st.env)}
	req := contracts.DecisionRequest{
		State:     prompts.Judgment("step_given", vars).State,
		Questions: map[string]contracts.DecisionQuestion{"given": question("step_given", vars)},
	}
	given, ok := r.judge(ctx, st, req)["given"]

	return given, ok
}
