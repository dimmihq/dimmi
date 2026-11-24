/*
|--------------------------------------------------------------------------
| Extract And Action Steps
|--------------------------------------------------------------------------
|
| Two steps share building blocks with agents. An extract step reads the
| conversation with the AI decision step and saves the typed values it
| finds, each with an optional fallback for when the models are unsure;
| a value with no fallback that stays unknown takes the none path. A
| run action step calls one of the workspace's actions, the same ones
| agents and procedures call, with inputs filled from variables, and
| saves the outputs it names; a call that fails takes the failure path.
|
*/

package workflowspec

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

/*
The step kinds that share building blocks with agents.
*/
const (
	KindExtract   = "extract"
	KindRunAction = "run_action"
)

/*
The value types an extract step saves.
*/
const (
	ValueText    = "text"
	ValueNumber  = "number"
	ValueDate    = "date"
	ValueBoolean = "boolean"
	ValueChoice  = "choice"
)

/*
Limits of these steps.
*/
const (
	maxExtractFields = 10
	maxChoices       = 30
	maxActionInputs  = 30
	maxInputValue    = 1000
)

/*
automationKinds returns the extract and run action step kinds.
*/
func automationKinds() []Kind {
	return []Kind{
		{Key: KindExtract, Fixed: []string{OutcomeNone}, Ready: true},
		{Key: KindRunAction, Fixed: []string{OutcomeFailure}, Ready: true, Action: true},
	}
}

/*
ExtractSettings reads the conversation, or the customer's newest
message with Source last_message, for the values of Fields, the team's
Instructions saying how.
*/
type ExtractSettings struct {
	Instructions string         `json:"instructions,omitempty"`
	Source       string         `json:"source,omitempty"`
	Fields       []ExtractField `json:"fields"`
}

/*
ExtractField is one value to find: its key and words for the model,
its type and, for a choice, the values it may take. SaveTo is where it
goes, run.vars.<key> when empty; Fallback is saved when the models are
unsure, and without one an unknown value takes the none path.
*/
type ExtractField struct {
	Key         string          `json:"key"`
	Label       string          `json:"label,omitempty"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"`
	Options     []string        `json:"options,omitempty"`
	SaveTo      string          `json:"save_to,omitempty"`
	Fallback    json.RawMessage `json:"fallback,omitempty"`
}

/*
Target is where the field's value is saved.
*/
func (f ExtractField) Target() string {
	if f.SaveTo != "" {
		return f.SaveTo
	}

	return "run.vars." + f.Key
}

/*
RunActionSettings calls the workspace action ActionID, a public action
id, with Inputs and saves the outputs Save names.
*/
type RunActionSettings struct {
	ActionID string        `json:"action_id"`
	Inputs   []ActionInput `json:"inputs,omitempty"`
	Save     []ActionSave  `json:"save,omitempty"`
}

/*
ActionInput fills one of the action's inputs: Value holds variables,
such as {{contact.email}}, or a fixed value.
*/
type ActionInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

/*
ActionSave copies the action's output Output, one of its response
fields, to Field: run.vars.<key> or contact.custom.<key>.
*/
type ActionSave struct {
	Output string `json:"output"`
	Field  string `json:"field"`
}

/*
automationChecks returns the checks of these steps.
*/
func (c *checker) automationChecks() map[string]func(Step) {
	return map[string]func(Step){KindExtract: c.checkExtract, KindRunAction: c.checkRunAction}
}

/*
checkExtract checks the fields an extract step looks for.
*/
func (c *checker) checkExtract(s Step) {
	e, ok := decode[ExtractSettings](c, s)
	if !ok {
		return
	}

	if len(e.Fields) == 0 || len(e.Fields) > maxExtractFields {
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: "fields", Message: "Look for 1 to 10 values."})
	}

	if utf8.RuneCountInString(e.Instructions) > maxNote {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "instructions", Message: "This text is too long."})
	}

	if e.Source != "" {
		c.oneOf(s, "source", e.Source, "last_message", "conversation")
	}

	seen := map[string]bool{}
	for _, f := range e.Fields {
		c.extractField(s, f, seen)
	}
}

/*
extractField checks one value an extract step looks for.
*/
func (c *checker) extractField(s Step, f ExtractField, seen map[string]bool) {
	bad := func(message string) {
		c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Field: "fields", Message: message})
	}

	switch {
	case !keyPattern().MatchString(f.Key):
		bad("Name each value with lowercase letters, digits and _.")
	case seen[f.Key]:
		bad("Two values share the name " + f.Key + ".")
	case !slices.Contains(ValueTypes(), f.Type):
		bad("Pick the type of " + f.Key + ".")
	case utf8.RuneCountInString(f.Label) > maxLabel*2 || utf8.RuneCountInString(f.Description) > maxDescription:
		bad("Describe " + f.Key + " in 300 characters.")
	case f.SaveTo != "" && !saveTarget(f.SaveTo):
		bad("Save " + f.Key + " to run.vars.<key> or contact.custom.<key>.")
	case !choicesFit(f):
		bad("Give " + f.Key + " 1 to 30 different choices, and choices only to a choice.")
	case len(f.Fallback) > 0 && string(f.Fallback) != "null" && !FitsValue(f, f.Fallback):
		bad("The fallback of " + f.Key + " must be a " + f.Type + ".")
	}

	seen[f.Key] = true
}

/*
choicesFit reports whether a field's choices suit its type.
*/
func choicesFit(f ExtractField) bool {
	if f.Type != ValueChoice {
		return len(f.Options) == 0
	}

	if len(f.Options) == 0 || len(f.Options) > maxChoices {
		return false
	}

	seen := map[string]bool{}
	for _, o := range f.Options {
		o = strings.TrimSpace(o)
		if o == "" || utf8.RuneCountInString(o) > maxLabel*2 || seen[o] {
			return false
		}

		seen[o] = true
	}

	return true
}

/*
ValueTypes lists the value types.
*/
func ValueTypes() []string {
	return []string{ValueText, ValueNumber, ValueDate, ValueBoolean, ValueChoice}
}

/*
FitsValue reports whether a JSON value is one of the field's type.
*/
func FitsValue(f ExtractField, raw json.RawMessage) bool {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return false
	}

	switch f.Type {
	case ValueNumber:
		_, ok := v.(float64)
		return ok
	case ValueBoolean:
		_, ok := v.(bool)
		return ok
	case ValueDate:
		text, ok := v.(string)
		_, err := time.Parse(time.DateOnly, text)

		return ok && err == nil
	case ValueChoice:
		text, ok := v.(string)
		return ok && slices.Contains(f.Options, text)
	}

	text, ok := v.(string)

	return ok && strings.TrimSpace(text) != "" && utf8.RuneCountInString(text) <= maxInputValue
}

/*
checkRunAction checks the action a step calls, its inputs and what it
saves.
*/
func (c *checker) checkRunAction(s Step) {
	a, ok := decode[RunActionSettings](c, s)
	if !ok {
		return
	}

	c.required(s, "action_id", a.ActionID, "Pick the action to run.")

	if len(a.Inputs) > maxActionInputs {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "inputs", Message: "Fill 30 inputs at most."})
	}

	names := map[string]bool{}
	for _, in := range a.Inputs {
		if strings.TrimSpace(in.Name) == "" || names[in.Name] || utf8.RuneCountInString(in.Value) > maxInputValue {
			c.fail(Issue{
				Code: CodeInvalidSetting, StepID: s.ID, Field: "inputs",
				Message: "Fill each input once, 1000 characters at most.",
			})
		}

		names[in.Name] = true
		c.variables(s, "inputs", in.Value)
	}

	if len(a.Save) > maxSaves {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "save", Message: "Save 10 values at most."})
	}

	for _, sv := range a.Save {
		if strings.TrimSpace(sv.Output) == "" || !saveTarget(sv.Field) {
			c.fail(Issue{
				Code: CodeInvalidSetting, StepID: s.ID, Field: "save",
				Message: "Save an output of the action to run.vars.<key> or contact.custom.<key>.",
			})
		}
	}
}
