/*
|--------------------------------------------------------------------------
| Extracting Values
|--------------------------------------------------------------------------
|
| Yes-or-no and one-of-a-list values are choices, so the decision model
| answers them, each with "unknown" as a choice of its own. Text,
| numbers and dates need words back, which only the chat model writes,
| held to a schema where every value may be null: null is how it says
| it does not know, and it is told never to guess. A value that comes
| back in the wrong shape is asked for again, once, and left out when
| it is still wrong. Nothing is ever trimmed or converted into shape.
|
*/

package aidecision

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
Field is one value to pull out: its key, a name and a description the
team wrote, its type, and for a choice the values it may take.
*/
type Field struct {
	Key         string
	Label       string
	Description string
	Type        string
	Options     []string
}

/*
Extraction asks for the values of the fields.
*/
type Extraction struct {
	Subject
	Instructions string
	Fields       []Field
}

/*
Extracted holds the values the models were sure of, by key: a string
for text, a date (YYYY-MM-DD) and a choice, a float64 for a number and
a bool for yes or no. Unsure lists the keys left out, and By names the
model that answered.
*/
type Extracted struct {
	Values map[string]any
	Unsure []string
	By     string
}

/*
unknown is the choice that says the text does not tell.
*/
const unknown = "unknown"

/*
sureChoice is the least probability at which the decision model's
choice of a value counts.
*/
const sureChoice = 0.6

/*
maxText is the longest text value kept.
*/
const maxText = 500

/*
Extract pulls out the values of the fields.
*/
func (d *Decider) Extract(ctx context.Context, e Extraction) (Extracted, error) {
	out := Extracted{Values: map[string]any{}}
	if len(e.Fields) == 0 {
		return out, nil
	}

	left, answered := d.extractChoices(ctx, e, &out)

	if len(left) > 0 && d.Model != nil {
		if values, err := d.extractByModel(ctx, e, left); err == nil {
			answered, out.By = true, ByModel
			maps.Copy(out.Values, values)
		}
	}

	for _, f := range e.Fields {
		if _, ok := out.Values[f.Key]; !ok {
			out.Unsure = append(out.Unsure, f.Key)
		}
	}

	if !answered {
		return out, ErrUnavailable
	}

	return out, nil
}

/*
extractChoices lets the decision model settle the fields it can, and
returns the fields left for the chat model and whether it answered.
*/
func (d *Decider) extractChoices(ctx context.Context, e Extraction, out *Extracted) ([]Field, bool) {
	if d.Decision == nil {
		return e.Fields, false
	}

	values, sure, ok := d.extractByDecision(ctx, e)
	if !ok {
		return e.Fields, false
	}

	out.By = ByDecision
	maps.Copy(out.Values, values)

	return slices.DeleteFunc(slices.Clone(e.Fields), func(f Field) bool { return sure[f.Key] }), true
}

/*
extractByDecision asks the decision model for the yes-or-no and choice
fields. Sure names every field it settled, a value or a sure unknown,
so the chat model is not asked about them again; false means it never
answered.
*/
func (d *Decider) extractByDecision(ctx context.Context, e Extraction) (map[string]any, map[string]bool, bool) {
	questions := map[string]contracts.DecisionQuestion{}
	for _, f := range e.Fields {
		if f.Type == TypeBoolean || f.Type == TypeChoice {
			questions[f.Key] = choiceQuestion(f, e.Instructions)
		}
	}

	if len(questions) == 0 {
		return nil, nil, false
	}

	for range decideTries {
		res, err := d.Decision.Decide(ctx, contracts.DecisionRequest{State: untrusted(e.Context), Questions: questions})
		if err != nil || res == nil {
			continue
		}

		values, sure, ok := readChoices(res.Answers, questions)
		if ok {
			return values, sure, true
		}
	}

	return nil, nil, false
}

/*
choiceQuestion is a yes-or-no or choice field as a question for the
decision model.
*/
func choiceQuestion(f Field, instructions string) contracts.DecisionQuestion {
	choices := map[string]string{unknown: prompts.Task("decisions/extract_unknown", nil)}
	if f.Type == TypeBoolean {
		choices["yes"], choices["no"] = "Yes.", "No."
	} else {
		for _, o := range f.Options {
			choices[o] = o
		}
	}

	return contracts.DecisionQuestion{
		Type: contracts.DecisionChoice, Choices: choices,
		Instructions: prompts.Task("decisions/extract_question", prompts.Vars{
			"instructions": instructionsOf(instructions), "field": fieldLine(f),
		}),
	}
}

/*
readChoices reads the decision model's answers, false when one is not
among its choices. A sure unknown settles a field with no value.
*/
func readChoices(
	answers map[string]contracts.DecisionAnswer, questions map[string]contracts.DecisionQuestion,
) (map[string]any, map[string]bool, bool) {
	values, sure := map[string]any{}, map[string]bool{}

	for key, q := range questions {
		a, ok := answers[key]
		if _, listed := q.Choices[a.Choice]; !ok || !listed {
			return nil, nil, false
		}

		if a.Confidence < sureChoice {
			continue
		}

		sure[key] = true

		switch {
		case a.Choice == unknown:
		case q.Choices["yes"] != "" && (a.Choice == "yes" || a.Choice == "no"):
			values[key] = a.Choice == "yes"
		default:
			values[key] = a.Choice
		}
	}

	return values, sure, true
}

/*
extractByModel asks the chat model for the fields, and asks again
about the values that came back in the wrong shape.
*/
func (d *Decider) extractByModel(ctx context.Context, e Extraction, fields []Field) (map[string]any, error) {
	out := map[string]any{}
	asking := fields

	var err error

	for try := range decideTries {
		var answer map[string]json.RawMessage
		if err = d.askFields(ctx, e, asking, &answer); err != nil {
			if try == 0 {
				return nil, err
			}

			break
		}

		var wrong []Field

		for _, f := range asking {
			v, state := valueOf(f, answer[f.Key])
			switch state {
			case valueOK:
				out[f.Key] = v
			case valueWrong:
				wrong = append(wrong, f)
			}
		}

		if len(wrong) == 0 {
			break
		}

		asking = wrong
	}

	return out, nil
}

/*
askFields asks the chat model for the fields under a schema where each
may be null.
*/
func (d *Decider) askFields(ctx context.Context, e Extraction, fields []Field, out any) error {
	props, keys := map[string]any{}, make([]string, 0, len(fields))

	var lines strings.Builder

	for _, f := range fields {
		keys = append(keys, f.Key)
		props[f.Key] = fieldSchema(f)
		fmt.Fprintf(&lines, "- %s\n", fieldLine(f))
	}

	schema, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": keys, "properties": props,
	})
	if err != nil {
		return fmt.Errorf("write values schema: %w", err)
	}

	return d.Model.AskModel(ctx, ModelQuestion{
		Subject: withContext(e.Subject), SchemaName: "ai_values", Schema: schema,
		System: prompts.Task("decisions/extract", prompts.Vars{
			"instructions": instructionsOf(e.Instructions), "fields": strings.TrimSpace(lines.String()),
		}),
	}, out)
}

/*
fieldSchema is the schema of one field's value, null allowed.
*/
func fieldSchema(f Field) map[string]any {
	switch f.Type {
	case TypeNumber:
		return map[string]any{"type": []string{"number", "null"}}
	case TypeBoolean:
		return map[string]any{"type": []string{"boolean", "null"}}
	case TypeChoice:
		enum := make([]any, 0, len(f.Options)+1)
		for _, o := range f.Options {
			enum = append(enum, o)
		}

		return map[string]any{"type": []string{"string", "null"}, "enum": append(enum, nil)}
	case TypeDate:
		return map[string]any{"type": []string{"string", "null"}, "description": "A date as YYYY-MM-DD."}
	}

	return map[string]any{"type": []string{"string", "null"}}
}

/*
fieldLine is a field as the models read it: its key, type, name,
description and choices.
*/
func fieldLine(f Field) string {
	line := f.Key + " (" + f.Type + ")"
	if l := strings.TrimSpace(f.Label); l != "" && l != f.Key {
		line += ": " + cut(l, maxLabel)
	}

	if d := strings.TrimSpace(f.Description); d != "" {
		line += ". " + cut(d, maxDescription)
	}

	if f.Type == TypeChoice {
		line += ". One of: " + strings.Join(f.Options, ", ")
	}

	return line
}

/*
The states of a value the chat model gave.
*/
const (
	valueOK = iota
	valueUnknown
	valueWrong
)

/*
valueOf reads one value: a value of the field's type, unknown for null
or nothing, or wrong when it does not fit.
*/
func valueOf(f Field, raw json.RawMessage) (any, int) {
	var v any
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil || v == nil {
		return nil, valueUnknown
	}

	if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
		return nil, valueUnknown
	}

	return check(f, v)
}

/*
check reports whether a value fits the field: a value of its type
returns valueOK, and anything else is wrong, never converted.
*/
func check(f Field, v any) (any, int) {
	fit, ok := checks()[f.Type]
	if !ok {
		fit = checkText
	}

	if out, fits := fit(f, v); fits {
		return out, valueOK
	}

	return nil, valueWrong
}

/*
checks are the checks of each value type but text.
*/
func checks() map[string]func(Field, any) (any, bool) {
	return map[string]func(Field, any) (any, bool){
		TypeNumber: func(_ Field, v any) (any, bool) {
			n, ok := v.(float64)
			return n, ok && !math.IsNaN(n) && !math.IsInf(n, 0)
		},
		TypeBoolean: func(_ Field, v any) (any, bool) {
			b, ok := v.(bool)
			return b, ok
		},
		TypeDate: func(_ Field, v any) (any, bool) {
			s, ok := v.(string)
			_, err := time.Parse(time.DateOnly, s)

			return s, ok && err == nil
		},
		TypeChoice: func(f Field, v any) (any, bool) {
			s, ok := v.(string)
			return s, ok && slices.Contains(f.Options, s)
		},
	}
}

/*
checkText checks a text value, kept to its first words.
*/
func checkText(_ Field, v any) (any, bool) {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return nil, false
	}

	return cut(strings.TrimSpace(s), maxText), true
}

/*
Fits reports whether a value fits the field.
*/
func Fits(f Field, v any) bool {
	_, state := check(f, v)

	return state == valueOK
}
