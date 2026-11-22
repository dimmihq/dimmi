/*
|--------------------------------------------------------------------------
| Rendering
|--------------------------------------------------------------------------
|
| How dimmi reads a procedure: its name, when it applies, the steps on
| the path numbered in order, each chip written out as the tool to call,
| the article to cite or the team to hand over to, and when to hand
| over. A branch code decided names the side it took and nothing of the
| other; one that waits says which action to call first. While a
| procedure is under way each step also carries its status.
|
*/

package procedure

import (
	"slices"
	"strings"
)

/*
Render writes a procedure as dimmi reads it with nothing called yet,
naming the tool each action calls; an action missing from tools is not
available.
*/
func Render(name string, d Definition, tools map[string]string) string {
	in := Input{Tools: tools}

	return RenderPlan(name, d, Resolve(d, in), in)
}

/*
RenderPlan writes a resolved procedure.
*/
func RenderPlan(name string, d Definition, plan Plan, in Input) string {
	var b strings.Builder
	b.WriteString("Procedure: " + name + "\nUse when: " + strings.TrimSpace(d.Description) + "\n\nSteps:\n")
	for _, n := range plan.Nodes {
		indent := strings.Repeat("   ", n.Depth)
		if n.Heading != "" {
			b.WriteString(indent + n.Heading + "\n")
		}
		b.WriteString(indent + n.Label + ". " + line(n, plan, in) + "\n")
	}

	if len(d.Handover) > 0 {
		b.WriteString("\nHand over to a teammate when:\n")
		for _, h := range d.Handover {
			b.WriteString("- " + strings.TrimSpace(h) + "\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

/*
line writes one step.
*/
func line(n Node, plan Plan, in Input) string {
	s := n.Step
	words := strings.TrimSpace(chips(s, in))
	switch {
	case n.Status == StatusLocked:
		head := "Decide"
		if words != "" {
			head += ": " + words
		}
		return head + " [LOCKED: call " + strings.Join(n.Needs, " and ") + " first to unlock the next steps]"
	case n.Deterministic:
		side := "true, so follow these steps"
		if n.Branch == "else" {
			side = "false, so follow these steps"
		}
		rule := Describe(*s.Condition)
		if words != "" {
			rule = words + " (" + rule + ")"
		}
		return "Decided: " + rule + " is " + side + "." + childless(n, plan)
	case s.Kind == KindSubProcedure:
		head := "Follow the sub-procedure"
		if words != "" {
			head += ": " + words
		}
		return head + mark(n, plan, in)
	}

	switch s.Kind {
	case KindCondition:
		words = "Check: " + words
	case KindAction:
		words = "Do: " + words
	}

	return words + notes(s, in) + mark(n, plan, in)
}

/*
childless notes a decided branch whose side has no steps.
*/
func childless(n Node, plan Plan) string {
	for _, other := range plan.Nodes {
		if strings.HasPrefix(other.Label, n.Label+".") {
			return ""
		}
	}

	return " There are no steps for this case: go on."
}

/*
chips writes a step's words with each marker spelled out.
*/
func chips(s Step, in Input) string {
	text := s.Text
	for _, m := range slices.Backward(Markers(text)) {
		text = text[:m.Start] + chip(s, m.Kind, m.Ref, in) + text[m.End:]
	}

	return text
}

/*
chip writes one chip.
*/
func chip(s Step, kind, ref string, in Input) string {
	b := bindingOf(s, kind, ref)
	label := labelOf(b, in)
	switch kind {
	case BindAction:
		return actionChip(b, label, in)
	case BindAttribute:
		return label
	case BindArticle:
		return "the help article \"" + label + "\""
	case BindHandoff:
		return "the " + label + " team (hand over with the handoff tool)"
	case BindTag:
		return "the tag \"" + label + "\" (add it with triage_conversation)"
	case BindTicketType:
		return "a \"" + label + "\" ticket (open it with open_ticket)"
	case BindSubProcedure:
		return "the \"" + label + "\" sub-procedure (its steps follow)"
	}

	return label
}

/*
actionChip names the tool an action chip calls, and how strictly.
*/
func actionChip(b Binding, label string, in Input) string {
	tool, ok := in.Tools[b.Ref]
	if !ok {
		return label + unavailableNote(b, in)
	}

	if b.Execution == ExecConditional {
		return "the " + tool + " tool (only when it applies)"
	}

	return "the " + tool + " tool (required)"
}

/*
notes adds a note for each action the step takes that its words do not
show as a chip, as a legacy action step does.
*/
func notes(s Step, in Input) string {
	var out strings.Builder
	shown := Markers(s.Text)
	for _, b := range s.Actions() {
		inline := false
		for _, m := range shown {
			inline = inline || (m.Kind == BindAction && m.Ref == b.Ref)
		}
		if inline {
			continue
		}
		tool, ok := in.Tools[b.Ref]
		if !ok {
			out.WriteString(unavailableNote(b, in))
			continue
		}
		out.WriteString(" (call the " + tool + " tool)")
	}

	return out.String()
}

/*
unavailableNote says an action cannot be called now, and what to do.
*/
func unavailableNote(b Binding, in Input) string {
	why := ""
	if reason := in.Unavailable[b.Ref]; reason != "" {
		why = " because " + strings.ReplaceAll(reason, "_", " ")
	}

	if b.OnMissing == MissingContinue {
		return " (this action is not available right now" + why + ": skip it and go on)"
	}

	return " (this action is not available right now" + why + ": hand over instead)"
}

/*
mark is a step's status while the procedure is under way.
*/
func mark(n Node, plan Plan, in Input) string {
	if !in.Marked {
		return ""
	}

	for _, b := range n.Step.Actions() {
		if waits := plan.Prerequisites(b.Ref, in.Called, in.Tools); len(waits) > 0 {
			names := make([]string, 0, len(waits))
			for _, w := range waits {
				names = append(names, in.Tools[w])
			}
			return " [LOCKED: needs " + strings.Join(names, ", ") + "]"
		}
	}

	switch n.Status {
	case StatusDone:
		return " [DONE]"
	case StatusActive:
		return " [ACTIVE]"
	case StatusEvaluated:
		return " [NOT TAKEN]"
	}

	return ""
}

/*
bindingOf finds a step's binding for a chip.
*/
func bindingOf(s Step, kind, ref string) Binding {
	for _, b := range s.Bindings {
		if b.Kind == kind && b.Ref == ref {
			return b
		}
	}

	return Binding{Kind: kind, Ref: ref}
}

/*
labelOf names what a chip points at: its current name, the name it had
when written, or its reference.
*/
func labelOf(b Binding, in Input) string {
	if l := in.Labels[b.Kind+":"+b.Ref]; l != "" {
		return l
	}

	if b.Label != "" {
		return b.Label
	}

	return b.Ref
}
