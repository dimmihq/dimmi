/*
|--------------------------------------------------------------------------
| Extract And Action Steps
|--------------------------------------------------------------------------
|
| Two steps share building blocks with agents. An extract step reads the
| person with the AI decision step and writes the typed values it finds
| to their attributes, a fallback standing in for a value the models
| were unsure of; one with no fallback sends them down the fallback
| path, or on when it leads nowhere. A run action step calls one of the
| workspace's actions, with inputs filled from the person, and writes
| the outputs it names to their attributes; a call that fails takes the
| failed path, or goes on when that leads nowhere.
|
*/

package series

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

/*
The kinds of step that share building blocks with agents.
*/
const (
	KindExtract   = "extract"
	KindRunAction = "run_action"
)

/*
The value types an extract step writes.
*/
const (
	ValueText    = "text"
	ValueNumber  = "number"
	ValueDate    = "date"
	ValueBoolean = "boolean"
	ValueChoice  = "choice"
)

/*
ExtractSettings reads the person for the values of Fields, the team's
Prompt saying how.
*/
type ExtractSettings struct {
	Prompt string         `json:"prompt,omitempty"`
	Fields []ExtractField `json:"fields"`
}

/*
ExtractField is one value to find: the attribute it is written to,
words for the model, its type, for a choice the values it may take,
and the fallback written when the models are unsure.
*/
type ExtractField struct {
	Key         string          `json:"key"`
	Label       string          `json:"label,omitempty"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"`
	Options     []string        `json:"options,omitempty"`
	Fallback    json.RawMessage `json:"fallback,omitempty"`
}

/*
RunActionSettings calls the workspace action ActionID with Inputs and
writes the outputs Save names to the person's attributes.
*/
type RunActionSettings struct {
	ActionID string        `json:"action_id"`
	Inputs   []ActionInput `json:"inputs,omitempty"`
	Save     []ActionSave  `json:"save,omitempty"`
}

/*
ActionInput fills one input: Value is a fixed value or a detail of the
person, such as {{contact.email}} or {{contact.attr.plan}}.
*/
type ActionInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

/*
ActionSave writes the output Output, one of the action's response
fields, to the attribute Attribute.
*/
type ActionSave struct {
	Output    string `json:"output"`
	Attribute string `json:"attribute"`
}

/*
valueTypes lists the value types.
*/
func valueTypes() []string {
	return []string{ValueText, ValueNumber, ValueDate, ValueBoolean, ValueChoice}
}

/*
extract checks the values an extract step looks for.
*/
func (c *checker) extract(n Node) {
	s := settingsOf[ExtractSettings](n)
	if len(s.Fields) == 0 || len(s.Fields) > 10 {
		c.add("extract_fields_invalid", at(n.ID, "fields"), "Look for 1 to 10 values.")
	}

	seen := map[string]bool{}
	for _, f := range s.Fields {
		switch {
		case !attributeKey.MatchString(f.Key) || seen[f.Key]:
			c.add("extract_key_invalid", at(n.ID, "fields"), "Write each value to a different attribute.")
		case !slices.Contains(valueTypes(), f.Type):
			c.add("extract_type_invalid", at(n.ID, "fields"), "Choose the type of %s.", f.Key)
		case f.Type == ValueChoice && !choicesFit(f.Options), f.Type != ValueChoice && len(f.Options) > 0:
			c.add("extract_choices_invalid", at(n.ID, "fields"), "Give %s 1 to 30 different choices.", f.Key)
		case len(f.Fallback) > 0 && string(f.Fallback) != "null" && !fits(f, f.Fallback):
			c.add("extract_fallback_invalid", at(n.ID, "fields"), "The fallback of %s must be a %s.", f.Key, f.Type)
		}

		seen[f.Key] = true
	}
}

/*
runAction checks the action a step calls and what it writes.
*/
func (c *checker) runAction(n Node) {
	s := settingsOf[RunActionSettings](n)
	if strings.TrimSpace(s.ActionID) == "" {
		c.add("action_missing", at(n.ID, "action_id"), "Choose the action to run.")
	}

	names := map[string]bool{}
	for _, in := range s.Inputs {
		if strings.TrimSpace(in.Name) == "" || names[in.Name] || utf8.RuneCountInString(in.Value) > 1000 {
			c.add("action_input_invalid", at(n.ID, "inputs"), "Fill each input once, 1000 characters at most.")
		}

		names[in.Name] = true
	}

	for _, sv := range s.Save {
		if strings.TrimSpace(sv.Output) == "" || !attributeKey.MatchString(sv.Attribute) {
			c.add("action_save_invalid", at(n.ID, "save"), "Write each output to an attribute.")
		}
	}
}

/*
choicesFit reports whether a choice's values are 1 to 30 different
words.
*/
func choicesFit(options []string) bool {
	if len(options) == 0 || len(options) > 30 {
		return false
	}

	seen := map[string]bool{}
	for _, o := range options {
		if strings.TrimSpace(o) == "" || seen[o] {
			return false
		}

		seen[o] = true
	}

	return true
}

/*
fits reports whether a JSON value is one of the field's type.
*/
func fits(f ExtractField, raw json.RawMessage) bool {
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

	return ok && strings.TrimSpace(text) != ""
}
