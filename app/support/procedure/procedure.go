/*
|--------------------------------------------------------------------------
| Procedures
|--------------------------------------------------------------------------
|
| A procedure is plain JSON: when it applies, its steps in order and
| when to hand over. A step is an instruction, a condition the agent
| weighs before going on, an action the agent takes through one of the
| workspace's own actions, a branch dimmi decides itself from a rule, or
| a reusable sub-procedure. A step's words may carry inline chips, kept
| in the text as markers such as [[action:act_...]] and described in the
| step's bindings, so the runtime never guesses what a chip meant. A
| working copy may be unfinished; only a definition with no problems can
| be published, so dimmi never follows a half-written procedure.
|
| Chips, rules, nested steps, scopes and enforcement are all optional: a
| definition that uses none of them reads, checks and renders exactly as
| it always has.
|
| The text dimmi reads is written by the team, so it is trusted as the
| workspace's own guidance, unlike anything a customer typed.
|
*/

package procedure

import (
	"bytes"
	"encoding/json"
	"fmt"
)

/*
The limits of a procedure: top-level steps, every step counted nested
ones included, the words of a step, when it applies, the handovers, how
deep then and else steps nest, the rules of one condition, and how
many sub-procedures may nest inside each other.
*/
const (
	MaxSteps       = 30
	MaxAllSteps    = 80
	MaxStepText    = 1000
	MaxDescription = 500
	MaxHandover    = 10
	MaxHandoverLen = 300
	MaxNesting     = 2
	MaxRules       = 10
	MaxSubDepth    = 3
	MaxRetries     = 3
)

/*
The step kinds.
*/
const (
	KindInstruction  = "instruction"
	KindCondition    = "condition"
	KindAction       = "action"
	KindBranch       = "branch"
	KindSubProcedure = "sub_procedure"
)

/*
The binding kinds: what an inline chip names.
*/
const (
	BindAction       = "action"
	BindAttribute    = "attribute"
	BindArticle      = "article"
	BindHandoff      = "handoff"
	BindTag          = "tag"
	BindTicketType   = "ticket_type"
	BindSubProcedure = "sub_procedure"
)

/*
How an action chip is enforced, what happens when it cannot be offered,
and what a broken rule does.
*/
const (
	ExecRequired    = "required"
	ExecConditional = "conditional"
	MissingHandoff  = "handoff"
	MissingContinue = "continue"
	ViolateHandoff  = "handoff"
	ViolateWarn     = "warn_only"
)

/*
Definition is a procedure: Description says when it applies, Steps are
followed in order, Handover lists when to hand the conversation to a
person. Scope narrows where it applies and HandoverRule routes the
handovers it causes.
*/
type Definition struct {
	Description  string          `json:"description"`
	Scope        *Scope          `json:"scope,omitempty"`
	Steps        []Step          `json:"steps"`
	Handover     []string        `json:"handover"`
	HandoverRule *HandoverConfig `json:"handover_rule,omitempty"`
}

/*
Step is one step: its id, kind and words, and for a legacy action step
the public id of the workspace action it takes. Bindings describe the
chips in its words; Enforcement says what must run before its actions;
Condition is the rule of a branch, deciding between ThenSteps and
ElseSteps; SubProcedure names the reusable procedure a sub_procedure
step follows.
*/
type Step struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	Text         string         `json:"text"`
	ActionID     string         `json:"action_id,omitempty"`
	Bindings     []Binding      `json:"bindings,omitempty"`
	Enforcement  *Enforcement   `json:"enforcement,omitempty"`
	Condition    *ConditionExpr `json:"condition,omitempty"`
	ThenSteps    []Step         `json:"then_steps,omitempty"`
	ElseSteps    []Step         `json:"else_steps,omitempty"`
	SubProcedure string         `json:"sub_procedure_id,omitempty"`
}

/*
Binding is one chip: what it names and, for an action, whether dimmi
must call it and what to do when it cannot be offered. Label is the
name it had when written, shown should the thing be renamed; Priority
goes with a handoff chip.
*/
type Binding struct {
	ID        string `json:"id,omitempty"`
	Kind      string `json:"kind"`
	Ref       string `json:"ref"`
	Label     string `json:"label,omitempty"`
	Execution string `json:"execution,omitempty"`
	OnMissing string `json:"on_missing,omitempty"`
	Priority  string `json:"priority,omitempty"`
}

/*
Enforcement is what must succeed before a step's actions run, how many
times dimmi is told to call a skipped required action, and what follows
when it still does not.
*/
type Enforcement struct {
	Requires    []string `json:"requires,omitempty"`
	MaxRetries  int      `json:"max_retries,omitempty"`
	OnViolation string   `json:"on_violation,omitempty"`
}

/*
Issue is one problem with a definition, pointing at its step.
*/
type Issue struct {
	Code    string `json:"code"`
	StepID  string `json:"step_id,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

/*
Empty is a new procedure's definition.
*/
func Empty() Definition {
	return Definition{Steps: []Step{}, Handover: []string{}}
}

/*
Parse reads a definition, refusing unknown keys.
*/
func Parse(raw json.RawMessage) (Definition, error) {
	d := Empty()
	if len(bytes.TrimSpace(raw)) == 0 {
		return d, nil
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("read procedure: %w", err)
	}

	if d.Steps == nil {
		d.Steps = []Step{}
	}

	if d.Handover == nil {
		d.Handover = []string{}
	}

	return d, nil
}

/*
Walk calls fn on every step, nested then and else steps included, in
reading order, with how deep the step sits.
*/
func Walk(steps []Step, fn func(s Step, depth int)) {
	walkFrom(steps, 0, fn)
}

/*
walkFrom walks steps that sit depth levels deep.
*/
func walkFrom(steps []Step, depth int, fn func(s Step, depth int)) {
	for _, s := range steps {
		fn(s, depth)
		walkFrom(s.ThenSteps, depth+1, fn)
		walkFrom(s.ElseSteps, depth+1, fn)
	}
}
