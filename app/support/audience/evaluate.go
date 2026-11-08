/*
|--------------------------------------------------------------------------
| Evaluating Rules
|--------------------------------------------------------------------------
|
| One walk answers three questions. Evaluate says yes or no with every
| value at hand. Partial plugs in what is known and returns what is
| left, so the server can decide everything it knows and hand the
| browser only the page conditions, never an attribute name or value.
| Explain records every condition's result, so "why didn't this run?"
| has an answer down to the leaf.
|
| Unknown values follow three-valued logic: false in an "all" group
| decides it, true in an "any" group decides it, anything else stays
| open. Evaluate treats a rule still open at the end as not matched.
|
*/

package audience

import (
	"encoding/json"
	"time"
)

/*
Truth is a three-valued result.
*/
type Truth int

/*
The results of a condition: false, true, or not decidable from what is
known.
*/
const (
	False Truth = iota
	True
	Unknown
)

/*
String names the result for traces.
*/
func (t Truth) String() string {
	if t == True {
		return "true"
	}

	if t == Unknown {
		return "unknown"
	}

	return "false"
}

/*
maxSavedDepth bounds how deep saved audiences may reference each other
while evaluating, so a cycle that slipped past saving still ends.
*/
const maxSavedDepth = 3

/*
Facts gives the value of a field. A missing value is not an error: the
field is simply absent for this person.
*/
type Facts interface {
	/*
		Lookup returns the field's value and whether there is one.
	*/
	Lookup(key string) (any, bool)
}

/*
Values is a ready map of field keys to values.
*/
type Values map[string]any

/*
Lookup returns the value of the key, absent when nil.
*/
func (v Values) Lookup(key string) (any, bool) {
	value, ok := v[key]
	if !ok || value == nil {
		return nil, false
	}

	return value, true
}

/*
Env is what evaluation reads: the facts, the current time and the
workspace's location for calendar days. Unknown marks fields that
cannot be known here, such as page fields on the server. Saved finds a
saved audience's program by id.
*/
type Env struct {
	Facts    Facts
	Now      time.Time
	Location *time.Location
	Unknown  func(f Field) bool
	Saved    func(id string) (*Program, bool)
	depth    int
}

/*
LeafResult is one condition's result in an explanation: the condition,
the value the person actually has, how far that value is trusted, and
whether the condition held.
*/
type LeafResult struct {
	Path   string          `json:"path"`
	Field  string          `json:"field"`
	Op     string          `json:"op"`
	Value  json.RawMessage `json:"value,omitempty"`
	Actual any             `json:"actual"`
	Trust  Trust           `json:"trust"`
	Result string          `json:"result"`
}

/*
Explanation is the result of a rule with every condition's result.
*/
type Explanation struct {
	Result Truth
	Leaves []LeafResult
}

/*
FirstFailing returns the first condition that was false, if any.
*/
func (e Explanation) FirstFailing() (LeafResult, bool) {
	for _, leaf := range e.Leaves {
		if leaf.Result == False.String() {
			return leaf, true
		}
	}

	return LeafResult{}, false
}

/*
Evaluate reports whether the audience matches with every fact at hand.
*/
func (p *Program) Evaluate(env Env) bool {
	t, _ := p.Partial(env)
	return t == True
}

/*
Partial evaluates what is known and returns the rule still open when the
result depends on unknown fields.
*/
func (p *Program) Partial(env Env) (Truth, *Rule) {
	if p == nil || p.root == nil {
		return True, nil
	}

	return eval(p.root, env, nil)
}

/*
Explain evaluates the audience and records every condition's result.
*/
func (p *Program) Explain(env Env) Explanation {
	if p == nil || p.root == nil {
		return Explanation{Result: True}
	}

	var leaves []LeafResult
	t, _ := eval(p.root, env, &leaves)

	return Explanation{Result: t, Leaves: leaves}
}

/*
eval walks one node. Every child is evaluated, not only until the
result is known, so an explanation lists every condition.
*/
func eval(n *node, env Env, rec *[]LeafResult) (Truth, *Rule) {
	switch n.kind {
	case "not":
		t, residual := eval(n.children[0], env, rec)
		if t == Unknown {
			return Unknown, &Rule{Not: residual}
		}

		return invert(t), nil
	case "all", "any":
		return evalGroup(n, env, rec)
	}

	t := evalLeaf(n, env)
	if rec != nil {
		*rec = append(*rec, leafResult(n, env, t))
	}

	if t == Unknown {
		leaf := n.original
		return Unknown, &leaf
	}

	return t, nil
}

/*
leafResult records one condition with the value it was decided on.
*/
func leafResult(n *node, env Env, t Truth) LeafResult {
	out := LeafResult{
		Path: n.path, Field: n.key, Op: n.op, Value: n.original.Value, Trust: n.field.Trust, Result: t.String(),
	}

	if env.Facts != nil && n.field.Type != TypeSaved {
		if v, ok := env.Facts.Lookup(n.key); ok {
			out.Actual = v
		}
	}

	return out
}

/*
evalGroup combines children by three-valued logic: in an all group a
false child decides, in an any group a true child does.
*/
func evalGroup(n *node, env Env, rec *[]LeafResult) (Truth, *Rule) {
	decisive, settled := False, True
	if n.kind == "any" {
		decisive, settled = True, False
	}

	decided := false
	var open []Rule

	for _, child := range n.children {
		t, residual := eval(child, env, rec)
		if t == decisive {
			decided = true
		}

		if t == Unknown {
			open = append(open, *residual)
		}
	}

	switch {
	case decided:
		return decisive, nil
	case len(open) == 0:
		return settled, nil
	case len(open) == 1:
		return Unknown, &open[0]
	case n.kind == "any":
		return Unknown, &Rule{Any: open}
	}

	return Unknown, &Rule{All: open}
}

/*
evalLeaf decides one condition.
*/
func evalLeaf(n *node, env Env) Truth {
	if env.Unknown != nil && env.Unknown(n.field) {
		return Unknown
	}

	if n.field.Type == TypeSaved {
		return evalSaved(n, env)
	}

	var value any
	present := false

	if env.Facts != nil {
		value, present = env.Facts.Lookup(n.key)
	}

	normalized := normalize(n, value)
	if !present || normalized == nil {
		return truth(negative(n.op))
	}

	return truth(compare(n, normalized, env))
}

/*
evalSaved decides membership of saved audiences: in matches when any
listed audience matches, not_in when none does.
*/
func evalSaved(n *node, env Env) Truth {
	if env.Saved == nil || env.depth >= maxSavedDepth {
		return truth(n.op == OpNotIn)
	}

	inner := env
	inner.depth++
	result := False

	for _, id := range n.operand.texts {
		program, ok := env.Saved(id)
		if !ok {
			continue
		}

		t, _ := program.Partial(inner)
		if t == True {
			result = True
			break
		}

		if t == Unknown {
			result = Unknown
		}
	}

	if n.op == OpNotIn {
		return invert(result)
	}

	return result
}

/*
invert negates a known result and leaves unknown as it is.
*/
func invert(t Truth) Truth {
	if t == Unknown {
		return Unknown
	}

	return truth(t == False)
}

/*
truth turns a boolean into a result.
*/
func truth(b bool) Truth {
	return map[bool]Truth{true: True, false: False}[b]
}
