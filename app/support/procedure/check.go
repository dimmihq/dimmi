/*
|--------------------------------------------------------------------------
| Checking A Procedure
|--------------------------------------------------------------------------
|
| What stops a definition from being published, each problem pointing
| at its step: when it applies, the number of steps, every step written,
| every chip described, every rule complete, nesting kept shallow and
| every prerequisite naming something the procedure takes.
|
*/

package procedure

import (
	"slices"
	"strings"
	"unicode/utf8"
)

/*
Check lists what stops the definition from being published.
*/
func Check(d Definition) []Issue {
	var out []Issue
	if n := utf8.RuneCountInString(strings.TrimSpace(d.Description)); n == 0 || n > MaxDescription {
		out = append(out, Issue{
			Code: "missing_setting", Field: "description",
			Message: "Say when dimmi should use this, in 500 characters.",
		})
	}

	if len(d.Steps) == 0 || len(d.Steps) > MaxSteps {
		out = append(out, Issue{Code: "limit_exceeded", Field: "steps", Message: "Write 1 to 30 steps."})
	}

	all := 0
	Walk(d.Steps, func(Step, int) { all++ })
	if all > MaxAllSteps {
		out = append(out, Issue{
			Code: "limit_exceeded", Field: "steps", Message: "Write at most 80 steps, nested ones included.",
		})
	}

	if len(d.Handover) > MaxHandover {
		out = append(out, Issue{Code: "limit_exceeded", Field: "handover", Message: "List at most 10 handovers."})
	}

	for _, h := range d.Handover {
		if n := utf8.RuneCountInString(strings.TrimSpace(h)); n == 0 || n > MaxHandoverLen {
			out = append(out, Issue{
				Code: "invalid_setting", Field: "handover", Message: "Write each handover in 1 to 300 characters.",
			})
		}
	}

	out = append(out, checkRule(d.HandoverRule)...)
	out = append(out, checkScope(d.Scope)...)

	seen := map[string]bool{}
	Walk(d.Steps, func(s Step, depth int) {
		out = append(out, checkStep(s, depth, seen)...)
	})

	return append(out, checkRequires(d)...)
}

/*
checkStep checks one step's id, kind, words, chips, rule and nesting.
*/
func checkStep(s Step, depth int, seen map[string]bool) []Issue {
	var out []Issue
	if s.ID == "" || seen[s.ID] {
		out = append(out, Issue{Code: "duplicate_id", StepID: s.ID, Message: "Every step needs its own id."})
	}

	seen[s.ID] = true
	out = append(out, checkWords(s)...)
	out = append(out, checkShape(s, depth)...)
	if s.Condition != nil {
		out = append(out, checkCondition(s.ID, *s.Condition)...)
	}

	out = append(out, checkBindings(s)...)

	return append(out, checkEnforcement(s)...)
}

/*
checkWords checks a step's kind, its words, and what its kind needs.
*/
func checkWords(s Step) []Issue {
	var out []Issue
	add := func(code, field, message string) {
		out = append(out, Issue{Code: code, StepID: s.ID, Field: field, Message: message})
	}

	if !slices.Contains(stepKinds(), s.Kind) {
		add("schema", "", "Use instruction, condition, action, branch or sub-procedure.")
	}

	text := utf8.RuneCountInString(strings.TrimSpace(s.Text))
	optional := s.Kind == KindBranch || s.Kind == KindSubProcedure
	if (text == 0 && !optional) || text > MaxStepText {
		add("missing_setting", "text", "Write the step in 1 to 1,000 characters.")
	}

	if s.Kind == KindAction && s.ActionID == "" && !hasActionChip(s) {
		add("missing_setting", "action_id", "Pick the action this step takes.")
	}

	if s.Kind == KindBranch && s.Condition == nil {
		add("missing_setting", "condition", "Write the rule this branch decides.")
	}

	if s.Kind == KindSubProcedure && s.SubProcedure == "" {
		add("missing_setting", "sub_procedure_id", "Pick the sub-procedure this step follows.")
	}

	return out
}

/*
checkShape checks only conditions have then and else steps, nested at
most MaxNesting deep.
*/
func checkShape(s Step, depth int) []Issue {
	if len(s.ThenSteps)+len(s.ElseSteps) == 0 {
		return nil
	}

	var out []Issue
	if s.Kind != KindBranch && s.Kind != KindCondition {
		out = append(out, Issue{
			Code: "schema", StepID: s.ID, Field: "then_steps",
			Message: "Only a condition can have then and else steps.",
		})
	}

	if depth >= MaxNesting {
		out = append(out, Issue{
			Code: "limit_exceeded", StepID: s.ID, Field: "then_steps",
			Message: "Nest then and else steps at most two levels deep.",
		})
	}

	return out
}

/*
stepKinds lists every step kind.
*/
func stepKinds() []string {
	return []string{KindInstruction, KindCondition, KindAction, KindBranch, KindSubProcedure}
}

/*
checkBindings checks each chip of a step and that every chip in the
words is described.
*/
func checkBindings(s Step) []Issue {
	var out []Issue
	kinds := []string{BindAction, BindAttribute, BindArticle, BindHandoff, BindTag, BindTicketType, BindSubProcedure}
	for _, b := range s.Bindings {
		switch {
		case !slices.Contains(kinds, b.Kind):
			out = append(
				out,
				Issue{Code: "schema", StepID: s.ID, Field: "bindings", Message: "A chip names nothing dimmi knows."},
			)
		case strings.TrimSpace(b.Ref) == "":
			out = append(
				out,
				Issue{Code: "missing_setting", StepID: s.ID, Field: "bindings", Message: "A chip is empty."},
			)
		case b.Execution != "" && b.Execution != ExecRequired && b.Execution != ExecConditional:
			out = append(out, Issue{
				Code: "invalid_setting", StepID: s.ID, Field: "bindings",
				Message: "Make an action required or conditional.",
			})
		case b.OnMissing != "" && b.OnMissing != MissingHandoff && b.OnMissing != MissingContinue:
			out = append(out, Issue{
				Code: "invalid_setting", StepID: s.ID, Field: "bindings",
				Message: "When an action is unavailable, hand over or continue.",
			})
		case b.Priority != "" && !slices.Contains(priorities(), b.Priority):
			out = append(out, Issue{
				Code: "invalid_setting", StepID: s.ID, Field: "bindings", Message: "Use low, medium, high or urgent.",
			})
		}
	}

	return out
}

/*
checkEnforcement checks a step's prerequisites and retry settings.
*/
func checkEnforcement(s Step) []Issue {
	e := s.Enforcement
	if e == nil {
		return nil
	}

	var out []Issue
	if e.MaxRetries < 0 || e.MaxRetries > MaxRetries {
		out = append(out, Issue{
			Code: "invalid_setting", StepID: s.ID, Field: "enforcement", Message: "Retry 1 to 3 times.",
		})
	}

	if e.OnViolation != "" && e.OnViolation != ViolateHandoff && e.OnViolation != ViolateWarn {
		out = append(out, Issue{
			Code: "invalid_setting", StepID: s.ID, Field: "enforcement",
			Message: "When a required action is skipped, hand over or only warn.",
		})
	}

	for _, r := range e.Requires {
		if strings.TrimSpace(r) == "" || r == s.ID {
			out = append(out, Issue{
				Code: "invalid_setting", StepID: s.ID, Field: "enforcement", Message: "A step cannot wait for itself.",
			})
		}
	}

	return out
}

/*
checkRequires checks that every prerequisite names a step or an action
of the procedure.
*/
func checkRequires(d Definition) []Issue {
	known := map[string]bool{}
	Walk(d.Steps, func(s Step, _ int) {
		known[s.ID] = true
		for _, b := range Normalize(Definition{Steps: []Step{s}}).Steps[0].Bindings {
			if b.Kind == BindAction {
				known[b.Ref] = true
			}
		}
	})

	var out []Issue
	Walk(d.Steps, func(s Step, _ int) {
		if s.Enforcement == nil {
			return
		}
		for _, r := range s.Enforcement.Requires {
			if r != "" && !known[r] {
				out = append(out, Issue{
					Code: "reference_missing", StepID: s.ID, Field: "enforcement",
					Message: "A step waits for an action this procedure does not take.",
				})
			}
		}
	})

	return out
}

/*
checkCondition checks a rule's mode and each comparison.
*/
func checkCondition(step string, c ConditionExpr) []Issue {
	var out []Issue
	if c.Mode != "all" && c.Mode != "any" {
		out = append(
			out,
			Issue{Code: "invalid_setting", StepID: step, Field: "condition", Message: "Match all or any rules."},
		)
	}

	if len(c.Rules) == 0 || len(c.Rules) > MaxRules {
		out = append(out, Issue{
			Code: "missing_setting", StepID: step, Field: "condition", Message: "Write 1 to 10 rules.",
		})
	}

	for _, r := range c.Rules {
		switch {
		case strings.TrimSpace(r.Field) == "":
			out = append(
				out,
				Issue{
					Code:    "missing_setting",
					StepID:  step,
					Field:   "condition",
					Message: "Pick a field for every rule.",
				},
			)
		case !slices.Contains(Operators(), r.Operator):
			out = append(
				out,
				Issue{
					Code:    "invalid_setting",
					StepID:  step,
					Field:   "condition",
					Message: "A rule compares in a way dimmi does not know.",
				},
			)
		case r.Value == nil && r.Operator != OpExists && r.Operator != OpNotExists:
			out = append(
				out,
				Issue{Code: "missing_setting", StepID: step, Field: "condition", Message: "Give every rule a value."},
			)
		}
	}

	return out
}

/*
checkRule checks the handover routing.
*/
func checkRule(r *HandoverConfig) []Issue {
	if r == nil || r.Priority == "" || slices.Contains(priorities(), r.Priority) {
		return nil
	}

	return []Issue{{Code: "invalid_setting", Field: "handover_rule", Message: "Use low, medium, high or urgent."}}
}

/*
checkScope checks the channels a procedure is narrowed to.
*/
func checkScope(s *Scope) []Issue {
	if s == nil {
		return nil
	}

	var out []Issue
	for _, c := range s.Channels {
		if !slices.Contains(Channels(), c) {
			out = append(
				out,
				Issue{
					Code:    "invalid_setting",
					Field:   "scope",
					Message: "A channel dimmi does not answer on is listed.",
				},
			)
		}
	}

	return out
}

/*
Channels lists the channels a procedure can be narrowed to.
*/
func Channels() []string {
	return []string{"widget", "email", "slack", "whatsapp", "instagram", "facebook", "api"}
}

/*
priorities lists the priorities a handover can set.
*/
func priorities() []string {
	return []string{"low", "medium", "high", "urgent"}
}

/*
hasActionChip reports whether a step's words or bindings name an
action.
*/
func hasActionChip(s Step) bool {
	for _, b := range s.Bindings {
		if b.Kind == BindAction && b.Ref != "" {
			return true
		}
	}

	for _, m := range Markers(s.Text) {
		if m.Kind == BindAction {
			return true
		}
	}

	return false
}
