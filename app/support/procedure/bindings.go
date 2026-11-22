/*
|--------------------------------------------------------------------------
| Chips
|--------------------------------------------------------------------------
|
| A chip in a step's words is a marker, [[kind:ref]], and the step's
| bindings say what each one means. Normalizing fills in what an older
| or hand-written definition leaves out: a legacy action step becomes a
| required action chip, a marker without a binding gets the default
| one, and every action binding gets its enforcement defaults. Nothing
| normalized is stored; the stored definition stays as it was written.
|
*/

package procedure

import (
	"regexp"
	"slices"
)

/*
markerPattern matches one chip marker in a step's words.
*/
var markerPattern = regexp.MustCompile(
	`\[\[(action|attribute|article|handoff|tag|ticket_type|sub_procedure):([^\]\s]+)\]\]`)

/*
Marker is one chip found in a step's words, with where it sits.
*/
type Marker struct {
	Kind  string
	Ref   string
	Start int
	End   int
}

/*
Markers lists the chips in a text, in order.
*/
func Markers(text string) []Marker {
	var out []Marker
	for _, m := range markerPattern.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, Marker{Kind: text[m[2]:m[3]], Ref: text[m[4]:m[5]], Start: m[0], End: m[1]})
	}

	return out
}

/*
Normalize returns the definition with every step's bindings complete.
*/
func Normalize(d Definition) Definition {
	d.Steps = normalizeSteps(d.Steps, nesting{})

	return d
}

/*
normalizeSteps completes the bindings of steps; steps nested under a
condition written in plain words take conditional actions unless the
team made them required.
*/
func normalizeSteps(steps []Step, under nesting) []Step {
	if steps == nil {
		return nil
	}

	out := make([]Step, len(steps))
	for i, s := range steps {
		s.Bindings = normalizeBindings(s, under)
		nested := nesting{plain: under.plain || (s.Kind == KindCondition && s.Condition == nil)}
		s.ThenSteps = normalizeSteps(s.ThenSteps, nested)
		s.ElseSteps = normalizeSteps(s.ElseSteps, nested)
		out[i] = s
	}

	return out
}

/*
nesting is where steps sit: plain marks steps under a condition written
in plain words.
*/
type nesting struct {
	plain bool
}

/*
normalizeBindings is a step's bindings with the legacy action, every
marker and every default in place.
*/
func normalizeBindings(s Step, under nesting) []Binding {
	out := slices.Clone(s.Bindings)
	has := func(kind, ref string) bool {
		return slices.ContainsFunc(out, func(b Binding) bool { return b.Kind == kind && b.Ref == ref })
	}

	if s.ActionID != "" && !has(BindAction, s.ActionID) {
		out = append(out, Binding{Kind: BindAction, Ref: s.ActionID, Execution: ExecRequired})
	}

	for _, m := range Markers(s.Text) {
		if !has(m.Kind, m.Ref) {
			out = append(out, Binding{Kind: m.Kind, Ref: m.Ref})
		}
	}

	if s.Kind == KindSubProcedure && s.SubProcedure != "" && !has(BindSubProcedure, s.SubProcedure) {
		out = append(out, Binding{Kind: BindSubProcedure, Ref: s.SubProcedure})
	}

	for i := range out {
		if out[i].Kind != BindAction {
			continue
		}
		if out[i].Execution == "" {
			out[i].Execution = defaultExecution(s, under)
		}
		if out[i].OnMissing == "" {
			out[i].OnMissing = MissingHandoff
		}
	}

	return out
}

/*
defaultExecution is how an action chip is enforced when the team did
not say: conditional in a condition written in plain words and in the
steps under it, required everywhere else.
*/
func defaultExecution(s Step, under nesting) string {
	plain := under.plain || (s.Kind == KindCondition && s.Condition == nil)

	return map[bool]string{true: ExecConditional, false: ExecRequired}[plain]
}

/*
Actions lists a step's action bindings.
*/
func (s Step) Actions() []Binding {
	var out []Binding
	for _, b := range s.Bindings {
		if b.Kind == BindAction {
			out = append(out, b)
		}
	}

	return out
}
