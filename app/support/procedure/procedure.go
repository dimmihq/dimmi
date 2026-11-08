/*
|--------------------------------------------------------------------------
| Procedures
|--------------------------------------------------------------------------
|
| A procedure is plain JSON: when it applies, its steps in order and
| when to hand over. A step is an instruction, a condition the agent
| weighs before going on, or an action the agent takes through one of
| the workspace's own actions. A working copy may be unfinished; only a
| definition with no problems can be published, so dimmi never follows
| a half-written procedure.
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
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

/*
The limits of a procedure.
*/
const (
	MaxSteps       = 30
	MaxStepText    = 1000
	MaxDescription = 500
	MaxHandover    = 10
	MaxHandoverLen = 300
)

/*
The step kinds.
*/
const (
	KindInstruction = "instruction"
	KindCondition   = "condition"
	KindAction      = "action"
)

/*
Definition is a procedure: Description says when it applies, Steps are
followed in order, Handover lists when to hand the conversation to a
person.
*/
type Definition struct {
	Description string   `json:"description"`
	Steps       []Step   `json:"steps"`
	Handover    []string `json:"handover"`
}

/*
Step is one step: its id, kind and words, and for an action step the
public id of the workspace action it takes.
*/
type Step struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	ActionID string `json:"action_id,omitempty"`
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

	return append(out, checkSteps(d.Steps)...)
}

/*
checkSteps checks each step's kind, words and action.
*/
func checkSteps(steps []Step) []Issue {
	var out []Issue
	seen := map[string]bool{}
	for _, s := range steps {
		if s.ID == "" || seen[s.ID] {
			out = append(out, Issue{Code: "duplicate_id", StepID: s.ID, Message: "Every step needs its own id."})
		}

		seen[s.ID] = true
		if s.Kind != KindInstruction && s.Kind != KindCondition && s.Kind != KindAction {
			out = append(out, Issue{Code: "schema", StepID: s.ID, Message: "Use instruction, condition or action."})
		}

		if n := utf8.RuneCountInString(strings.TrimSpace(s.Text)); n == 0 || n > MaxStepText {
			out = append(out, Issue{
				Code: "missing_setting", StepID: s.ID, Field: "text",
				Message: "Write the step in 1 to 1,000 characters.",
			})
		}

		if s.Kind == KindAction && s.ActionID == "" {
			out = append(out, Issue{
				Code: "missing_setting", StepID: s.ID, Field: "action_id", Message: "Pick the action this step takes.",
			})
		}
	}

	return out
}

/*
ActionIDs lists the actions the definition's steps take, once each.
*/
func ActionIDs(d Definition) []string {
	var out []string
	for _, s := range d.Steps {
		if s.Kind == KindAction && s.ActionID != "" && !slices.Contains(out, s.ActionID) {
			out = append(out, s.ActionID)
		}
	}

	return out
}

/*
Render writes a procedure as dimmi reads it: its name, numbered steps
naming the tool each action step calls, and when to hand over.
*/
func Render(name string, d Definition, tools map[string]string) string {
	var b strings.Builder
	b.WriteString("Procedure: " + name + "\nUse when: " + strings.TrimSpace(d.Description) + "\n\nSteps:\n")
	for i, s := range d.Steps {
		line := strings.TrimSpace(s.Text)
		switch s.Kind {
		case KindCondition:
			line = "Check: " + line
		case KindAction:
			line = "Do: " + line + actionNote(tools[s.ActionID])
		}

		b.WriteString(strconv.Itoa(i+1) + ". " + line + "\n")
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
actionNote names the tool an action step calls, or says it cannot be
taken now.
*/
func actionNote(tool string) string {
	if tool == "" {
		return " (this action is not available right now: hand over instead)"
	}

	return " (call the " + tool + " tool)"
}
