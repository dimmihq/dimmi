/*
|--------------------------------------------------------------------------
| The Path Through A Procedure
|--------------------------------------------------------------------------
|
| Before dimmi reads a procedure, code works out which of its steps
| apply now. A branch whose rule is decided keeps only the steps of the
| side it took; the other side is left out entirely, so dimmi never sees,
| and is never held to, an action on a branch not taken. A branch that
| waits for an action's result shows neither side until the action ran.
| A condition written in plain words keeps both sides for dimmi to
| weigh, and nothing under it is enforced. Sub-procedures are expanded
| in place, their step ids scoped under the step that follows them.
|
| The path is what every guardrail reads: the prerequisite gate, the
| check that required actions ran, the trace and the status markers.
|
*/

package procedure

import (
	"slices"
	"strconv"
	"strings"
)

/*
The status of a step on the path.
*/
const (
	StatusDone      = "done"
	StatusActive    = "active"
	StatusPending   = "pending"
	StatusLocked    = "locked"
	StatusEvaluated = "evaluated"
)

/*
Sub is a reusable procedure another one follows.
*/
type Sub struct {
	Name string
	Def  Definition
}

/*
Input is what resolving and rendering read: the rule facts, the actions
the conversation already called and those offered now, each by public
id, which of them only read, why an action is not offered, the
sub-procedures by public id, the names of the things chips point at,
whether to mark each step's status, and the public id of the procedure
itself, which its sub-procedures never expand again.
*/
type Input struct {
	Facts       Facts
	Called      map[string]bool
	Tools       map[string]string
	Reads       map[string]bool
	Unavailable map[string]string
	Subs        map[string]Sub
	Labels      map[string]string
	Marked      bool
	Root        string
}

/*
Node is one step on the path: its scoped id and number, how deep it
sits, the step, whether its actions are only conditional, the side a
branch took and whether code decided it, the tools a locked branch
waits for, its status, and the sub-procedure it belongs to.
*/
type Node struct {
	ID            string
	Label         string
	Depth         int
	Step          Step
	Conditional   bool
	Branch        string
	Deterministic bool
	Needs         []string
	Status        string
	Sub           string
	Heading       string
}

/*
Planned is one action on the path, in reading order: the step that
takes it, its enforcement, whether it only reads, the actions that must
run before it, and where on the path it sits.
*/
type Planned struct {
	ActionID    string
	StepID      string
	Label       string
	Execution   string
	OnMissing   string
	Conditional bool
	Read        bool
	Requires    []string
	MaxRetries  int
	OnViolation string
	Order       int
}

/*
Enforced reports whether dimmi must call the action.
*/
func (p Planned) Enforced() bool {
	return !p.Conditional && p.Execution == ExecRequired
}

/*
Plan is the path through a procedure: its steps and their actions, and
the actions of the steps left off it, under a branch that waits or the
side a branch did not take, which still wait for their prerequisites.
*/
type Plan struct {
	Nodes   []Node
	Actions []Planned
	Hidden  []Planned
}

/*
resolver walks a definition into a plan.
*/
type resolver struct {
	in      Input
	plan    Plan
	visited []string
}

/*
Resolve works out the path through a definition.
*/
func Resolve(d Definition, in Input) Plan {
	r := &resolver{in: in}
	if in.Root != "" {
		r.visited = []string{in.Root}
	}
	r.steps(Normalize(d).Steps, frame{})
	r.statuses()

	return r.plan
}

/*
frame is where a list of steps sits: the scope its ids are prefixed
with, the number its labels are prefixed with, how deep it is, whether
its actions are only conditional, and the sub-procedure it belongs to.
*/
type frame struct {
	scope       string
	label       string
	depth       int
	conditional bool
	sub         string
	subDepth    int
}

/*
steps adds a list of steps to the plan.
*/
func (r *resolver) steps(steps []Step, f frame) {
	for i, s := range steps {
		r.step(s, f, f.label+strconv.Itoa(i+1))
	}
}

/*
step adds one step and whatever it leads to.
*/
func (r *resolver) step(s Step, f frame, label string) {
	n := Node{
		ID: f.scope + s.ID, Label: label, Depth: f.depth, Step: s, Sub: f.sub,
		Conditional: f.conditional || (s.Kind == KindCondition && s.Condition == nil),
	}
	child := frame{
		scope: f.scope, label: label + ".", depth: f.depth + 1, conditional: f.conditional, sub: f.sub,
		subDepth: f.subDepth,
	}

	switch {
	case s.Condition != nil && (s.Kind == KindBranch || s.Kind == KindCondition):
		v := Evaluate(*s.Condition, r.in.Facts)
		n.Deterministic, n.Status = true, StatusEvaluated
		if !v.Known {
			n.Status, n.Needs = StatusLocked, v.Needs
			r.plan.Nodes = append(r.plan.Nodes, n)
			r.hide(s.ThenSteps, child)
			r.hide(s.ElseSteps, child)
			return
		}
		n.Branch = "else"
		taken, left := s.ElseSteps, s.ThenSteps
		if v.Value {
			n.Branch, taken, left = "then", s.ThenSteps, s.ElseSteps
		}
		r.add(n)
		r.steps(taken, child)
		r.hide(left, child)
	case s.Kind == KindCondition && len(s.ThenSteps)+len(s.ElseSteps) > 0:
		n.Branch = "both"
		r.add(n)
		child.conditional = true
		r.branchSide(s.ThenSteps, child, label+".", "If so:")
		r.branchSide(s.ElseSteps, child, label+".", "If not:")
	default:
		r.add(n)
	}

	r.expandSubs(s, f, label)
}

/*
branchSide adds one side of a condition written in plain words, its
first step carrying the side's heading.
*/
func (r *resolver) branchSide(steps []Step, f frame, prefix, heading string) {
	start := len(r.plan.Nodes)
	if heading == "If not:" {
		f.label = strings.TrimSuffix(prefix, ".") + "b."
	} else {
		f.label = strings.TrimSuffix(prefix, ".") + "a."
	}
	r.steps(steps, f)
	if start < len(r.plan.Nodes) {
		r.plan.Nodes[start].Heading = heading
	}
}

/*
expandSubs follows every sub-procedure a step names, in place, with its
step ids scoped under the step.
*/
func (r *resolver) expandSubs(s Step, f frame, label string) {
	for _, b := range s.Bindings {
		if b.Kind != BindSubProcedure {
			continue
		}
		sub, ok := r.in.Subs[b.Ref]
		if !ok || f.subDepth >= MaxSubDepth || slices.Contains(r.visited, b.Ref) {
			continue
		}
		r.visited = append(r.visited, b.Ref)
		r.steps(Normalize(sub.Def).Steps, frame{
			scope: f.scope + s.ID + "/", label: label + ".", depth: f.depth + 1, conditional: f.conditional,
			sub: b.Ref, subDepth: f.subDepth + 1,
		})
		r.visited = r.visited[:len(r.visited)-1]
	}
}

/*
hide keeps the actions of steps left off the path, nested ones
included, at the place of the branch that left them off.
*/
func (r *resolver) hide(steps []Step, f frame) {
	order := len(r.plan.Nodes) - 1
	Walk(steps, func(s Step, _ int) {
		n := Node{ID: f.scope + s.ID, Step: s, Conditional: true}
		r.plan.Hidden = append(r.plan.Hidden, r.planned(n, order)...)
	})
}

/*
add puts a node on the path, with its actions.
*/
func (r *resolver) add(n Node) {
	r.plan.Nodes = append(r.plan.Nodes, n)
	r.plan.Actions = append(r.plan.Actions, r.planned(n, len(r.plan.Nodes)-1)...)
}

/*
planned lists a node's actions with their enforcement.
*/
func (r *resolver) planned(n Node, order int) []Planned {
	var out []Planned
	scope := strings.TrimSuffix(n.ID, n.Step.ID)
	for _, b := range n.Step.Actions() {
		p := Planned{
			ActionID: b.Ref, StepID: n.ID, Label: n.Label, Execution: b.Execution, OnMissing: b.OnMissing,
			Conditional: b.Execution != ExecRequired, Read: r.in.Reads[b.Ref], MaxRetries: MaxRetries,
			OnViolation: ViolateHandoff, Order: order,
		}
		if e := n.Step.Enforcement; e != nil {
			if e.MaxRetries > 0 {
				p.MaxRetries = min(e.MaxRetries, MaxRetries)
			}
			if e.OnViolation != "" {
				p.OnViolation = e.OnViolation
			}
			p.Requires = r.requires(e.Requires, scope, b.Ref)
		}
		out = append(out, p)
	}

	return out
}

/*
requires turns a step's prerequisites, step ids or action ids, into the
actions that must run first.
*/
func (r *resolver) requires(refs []string, scope, self string) []string {
	var out []string
	for _, ref := range refs {
		for _, id := range r.actionsOf(ref, scope) {
			if id != self && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}

	return out
}

/*
actionsOf is the actions a prerequisite names: those of the step it
names, or the action itself.
*/
func (r *resolver) actionsOf(ref, scope string) []string {
	var out []string
	found := false
	for _, n := range r.plan.Nodes {
		if n.ID != scope+ref && n.ID != ref {
			continue
		}
		found = true
		for _, b := range n.Step.Actions() {
			out = append(out, b.Ref)
		}
	}

	if !found {
		return []string{ref}
	}

	return out
}

/*
statuses marks each step done, active, pending or locked. A step with
actions is done once all of them were called; a step without is done
once a step after it is.
*/
func (r *resolver) statuses() {
	nodes := r.plan.Nodes
	doneAfter := false
	for i := len(nodes) - 1; i >= 0; i-- {
		n := &nodes[i]
		if n.Status == StatusLocked {
			continue
		}
		actions := n.Step.Actions()
		done := len(actions) > 0
		for _, b := range actions {
			done = done && r.in.Called[b.Ref]
		}
		if len(actions) == 0 {
			done = doneAfter
		}
		if done {
			doneAfter = true
		}
		switch {
		case n.Status == StatusEvaluated:
		case done:
			n.Status = StatusDone
		default:
			n.Status = StatusPending
		}
	}

	for i := range nodes {
		if nodes[i].Status == StatusPending {
			nodes[i].Status = StatusActive
			break
		}
	}
}

/*
Prerequisites lists the actions that must run before an action and
have not: those its step waits for, and every required read the path
takes before it. An action the turn cannot offer is never waited for,
so a missing lookup cannot stall the procedure. An action under a
branch that waits, or on a side not taken, waits the same way from the
branch's place. An action the procedure never names waits for nothing.
*/
func (p Plan) Prerequisites(action string, called map[string]bool, offered map[string]string) []string {
	this, ok := p.locate(action)
	if !ok {
		return nil
	}

	var out []string
	add := func(id string) {
		if id == action || called[id] || slices.Contains(out, id) {
			return
		}
		if _, ok := offered[id]; ok {
			out = append(out, id)
		}
	}

	for _, id := range this.Requires {
		add(id)
	}

	at := slices.IndexFunc(p.Actions, func(a Planned) bool { return a.ActionID == action })
	for i, earlier := range p.Actions {
		before := (at >= 0 && i < at) || (at < 0 && earlier.Order <= this.Order)
		if before && earlier.Enforced() && earlier.Read {
			add(earlier.ActionID)
		}
	}

	return out
}

/*
locate returns where an action first sits: on the path, or else off it.
*/
func (p Plan) locate(action string) (Planned, bool) {
	for _, list := range [][]Planned{p.Actions, p.Hidden} {
		if i := slices.IndexFunc(list, func(a Planned) bool { return a.ActionID == action }); i >= 0 {
			return list[i], true
		}
	}

	return Planned{}, false
}

/*
Missing lists the required actions on the path that were not called
though dimmi could call them now: offered, and with every prerequisite
met. Each action is listed once, at its first step.
*/
func (p Plan) Missing(called map[string]bool, offered map[string]string) []Planned {
	var out []Planned
	for _, a := range p.Actions {
		if !a.Enforced() || called[a.ActionID] {
			continue
		}
		if _, ok := offered[a.ActionID]; !ok {
			continue
		}
		if len(p.Prerequisites(a.ActionID, called, offered)) > 0 {
			continue
		}
		if slices.ContainsFunc(out, func(o Planned) bool { return o.ActionID == a.ActionID }) {
			continue
		}
		out = append(out, a)
	}

	return out
}

/*
Unavailable lists the required actions on the path the turn cannot
offer and whose step says to hand over when that happens.
*/
func (p Plan) Unavailable(offered map[string]string) []Planned {
	var out []Planned
	for _, a := range p.Actions {
		if _, ok := offered[a.ActionID]; ok || !a.Enforced() || a.OnMissing != MissingHandoff {
			continue
		}
		out = append(out, a)
	}

	return out
}

/*
Find returns the node of a scoped step id.
*/
func (p Plan) Find(id string) (Node, bool) {
	i := slices.IndexFunc(p.Nodes, func(n Node) bool { return n.ID == id })
	if i < 0 {
		return Node{}, false
	}

	return p.Nodes[i], true
}
