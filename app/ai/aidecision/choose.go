/*
|--------------------------------------------------------------------------
| Choosing A Path
|--------------------------------------------------------------------------
|
| A choice lists the team's paths and one more, none, which is the right
| answer when nothing fits. The decision model picks one with how likely
| it is; the chat model picks one with low, medium or high. An answer
| outside the list is no answer: the model is asked again, and when it
| still cannot answer the caller takes its own no-match path.
|
*/

package aidecision

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
Option is one path a choice may take: its id, its name and what
belongs on it, all written by the team.
*/
type Option struct {
	ID          string
	Label       string
	Description string
}

/*
Choice asks which of the options fits, the team's instructions saying
how to tell.
*/
type Choice struct {
	Subject
	Instructions string
	Options      []Option
}

/*
Chosen is the option picked, empty when none fits, how sure the model
was and which model answered.
*/
type Chosen struct {
	OptionID   string
	Confidence string
	By         string
}

/*
none is the option that says no path fits.
*/
const none = "none"

/*
Limits on what the team's words add to a question.
*/
const (
	maxInstructions = 1000
	maxLabel        = 80
	maxDescription  = 300
	maxContext      = 8000
)

/*
Choose picks one of the options, or none.
*/
func (d *Decider) Choose(ctx context.Context, c Choice) (Chosen, error) {
	if len(c.Options) == 0 {
		return Chosen{}, nil
	}

	if d.Decision != nil {
		for range decideTries {
			if got, ok := d.chooseByDecision(ctx, c); ok {
				return got, nil
			}
		}
	}

	if d.Model == nil {
		return Chosen{}, ErrUnavailable
	}

	var last error

	// The model tries the workspace's other models itself; a fit answer
	// that is not on the list is asked again.
	for range decideTries {
		got, ok, err := d.chooseByModel(ctx, c)
		if ok {
			return got, nil
		}

		if err != nil {
			last = err

			break
		}
	}

	if last == nil {
		last = ErrUnavailable
	}

	return Chosen{}, fmt.Errorf("%w: %w", ErrUnavailable, last)
}

/*
chooseByDecision asks the decision model, false when it failed or gave
an option that is not on the list.
*/
func (d *Decider) chooseByDecision(ctx context.Context, c Choice) (Chosen, bool) {
	choices := map[string]string{none: prompts.Task("decisions/choose_none", nil)}
	for _, o := range c.Options {
		choices[o.ID] = optionLine(o)
	}

	res, err := d.Decision.Decide(ctx, contracts.DecisionRequest{
		State: untrusted(c.Context),
		Questions: map[string]contracts.DecisionQuestion{"path": {
			Type: contracts.DecisionChoice, Choices: choices,
			Instructions: prompts.Task("decisions/choose_question", prompts.Vars{
				"instructions": instructionsOf(c.Instructions),
			}),
		}},
	})
	if err != nil || res == nil {
		return Chosen{}, false
	}

	answer, ok := res.Answers["path"]
	if !ok || (answer.Choice != none && !hasOption(c.Options, answer.Choice)) {
		return Chosen{}, false
	}

	chosen := Chosen{Confidence: level(answer.Confidence), By: ByDecision}
	if answer.Choice != none {
		chosen.OptionID = answer.Choice
	}

	return chosen, true
}

/*
chooseByModel asks the chat model, false when it failed or answered
outside the list.
*/
func (d *Decider) chooseByModel(ctx context.Context, c Choice) (Chosen, bool, error) {
	ids := []string{none}

	var lines strings.Builder

	for _, o := range c.Options {
		ids = append(ids, o.ID)
		fmt.Fprintf(&lines, "- %s: %s\n", o.ID, optionLine(o))
	}

	schema, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"choice", "confidence"},
		"properties": map[string]any{
			"choice":     map[string]any{"type": "string", "enum": ids},
			"confidence": map[string]any{"type": "string", "enum": levels()},
		},
	})
	if err != nil {
		return Chosen{}, false, fmt.Errorf("write choice schema: %w", err)
	}

	var answer struct {
		Choice     string `json:"choice"`
		Confidence string `json:"confidence"`
	}

	err = d.Model.AskModel(ctx, ModelQuestion{
		Subject: withContext(c.Subject), SchemaName: "ai_choice", Schema: schema,
		System: prompts.Task("decisions/choose", prompts.Vars{
			"instructions": instructionsOf(c.Instructions), "options": strings.TrimSpace(lines.String()),
		}),
	}, &answer)
	if err != nil {
		return Chosen{}, false, err
	}

	if !slices.Contains(ids, answer.Choice) || !slices.Contains(levels(), answer.Confidence) {
		return Chosen{}, false, nil
	}

	chosen := Chosen{Confidence: answer.Confidence, By: ByModel}
	if answer.Choice != none {
		chosen.OptionID = answer.Choice
	}

	return chosen, true, nil
}

/*
SureEnough reports whether a confidence meets the least one asked for,
any confidence meeting none.
*/
func SureEnough(confidence, least string) bool {
	rank := map[string]int{ConfidenceLow: 1, ConfidenceMedium: 2, ConfidenceHigh: 3}

	return least == "" || rank[confidence] >= rank[least]
}

/*
levels are the confidence levels, lowest first.
*/
func levels() []string {
	return []string{ConfidenceLow, ConfidenceMedium, ConfidenceHigh}
}

/*
level turns the decision model's probability into a level.
*/
func level(p float64) string {
	switch {
	case p >= 0.8:
		return ConfidenceHigh
	case p >= 0.55:
		return ConfidenceMedium
	}

	return ConfidenceLow
}

/*
hasOption reports whether the id is one of the options.
*/
func hasOption(options []Option, id string) bool {
	return slices.ContainsFunc(options, func(o Option) bool { return o.ID == id })
}

/*
optionLine is an option as the models read it.
*/
func optionLine(o Option) string {
	line := cut(o.Label, maxLabel)
	if d := strings.TrimSpace(o.Description); d != "" {
		line += ". " + cut(d, maxDescription)
	}

	return line
}

/*
instructionsOf is the team's instructions, or a line saying there are
none.
*/
func instructionsOf(text string) string {
	if t := strings.TrimSpace(text); t != "" {
		return cut(t, maxInstructions)
	}

	return "(none)"
}

/*
withContext cuts the subject's text to what the models read.
*/
func withContext(s Subject) Subject {
	s.Context = newest(s.Context)

	return s
}

/*
untrusted wraps text a customer may have written, so a model reads it
as data and never as instructions.
*/
func untrusted(text string) string {
	text = strings.ReplaceAll(newest(text), "</untrusted>", "")

	return "<untrusted>\n" + text + "\n</untrusted>"
}

/*
cut keeps the first n runes of a text.
*/
func cut(text string, n int) string {
	if r := []rune(text); len(r) > n {
		return string(r[:n])
	}

	return text
}

/*
newest keeps the newest words of a text, as many as the models read.
*/
func newest(text string) string {
	if r := []rune(text); len(r) > maxContext {
		return string(r[len(r)-maxContext:])
	}

	return text
}
