/*
|--------------------------------------------------------------------------
| Dry Run Steps
|--------------------------------------------------------------------------
|
| How each kind of step goes in a dry run. A customer step reads the
| same settings and takes the same paths a run's does; an action only
| says what it would do, and changes the dry run's own copy of the
| conversation where a later step could read it. A kind this file does
| not know goes on along its own path: an action is recorded as what
| would run, anything else as not simulated.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
step runs one step of a dry run.
*/
func (d *dry) step(ctx context.Context, s workflowspec.Step) (outcome, error) {
	switch s.Kind {
	case workflowspec.KindMessage:
		return d.say(ctx, s, settings[workflowspec.MessageSettings](s).Text), nil
	case workflowspec.KindArticle:
		a := settings[workflowspec.ArticleSettings](s)
		return d.say(ctx, s, strings.TrimSpace(a.Text+"\n\n[Help article "+a.Article+"]")), nil
	case workflowspec.KindReplyTime:
		text := settings[workflowspec.ReplyTimeSettings](s).Text
		if text == "" {
			text = "The team will reply {{reply_time}}."
		}
		return d.say(ctx, s, strings.ReplaceAll(text, "{{reply_time}}", "soon")), nil
	case workflowspec.KindButtons:
		return d.buttons(ctx, s)
	case workflowspec.KindCollect:
		return d.collect(ctx, s)
	case workflowspec.KindBranch:
		return d.branch(ctx, s)
	case workflowspec.KindClassify:
		return d.classify(ctx, s), nil
	case workflowspec.KindWait:
		return outcome{outcome: outcomeElapsed, status: "skipped", detail: map[string]any{"wait": "skipped"}}, nil
	case workflowspec.KindDimmi:
		return d.handle(ctx, s)
	case workflowspec.KindHTTPRequest:
		return d.request(s), nil
	}

	return d.action(ctx, s), nil
}

/*
say sends a message to the customer.
*/
func (d *dry) say(ctx context.Context, s workflowspec.Step, text string) outcome {
	text = d.render(ctx, text)
	d.in.Driver.Said(ctx, DryMessage{StepID: s.ID, Text: text})

	return outcome{outcome: outcomeSent, acted: true, detail: map[string]any{"text": text}}
}

/*
buttons asks a question with buttons and takes the path of the answer:
a tapped button, a typed answer that names one, the typed path, or the
timeout path when the customer says nothing. A typed answer that
matches nothing is asked again once, then ends the run.
*/
func (d *dry) buttons(ctx context.Context, s workflowspec.Step) (outcome, error) {
	b := settings[workflowspec.ButtonsSettings](s)
	text := d.render(ctx, b.Text)
	labels := labels(choiceLabels(s))
	d.in.Driver.Said(ctx, DryMessage{StepID: s.ID, Text: text, Choices: labels})
	typing := workflowspec.On(b.AllowTyping, true)
	ask := DryAsk{
		StepID: s.ID, Kind: s.Kind, Question: text, Choices: labels, ChoicesOnly: !typing, CanWait: b.Timeout != "",
	}
	for try := range 2 {
		answer, err := d.in.Driver.Customer(ctx, ask)
		if err != nil {
			return outcome{}, err
		}
		if out, ok := answered(s, answer, b); ok {
			return out, nil
		}
		if try == 0 {
			retry := cmpText(b.RetryText, "Please reply with one of the numbers above.")
			d.in.Driver.Said(ctx, DryMessage{StepID: s.ID, Text: retry, Choices: labels})
		}
	}

	return outcome{end: EndNoChoice, outcome: "no_choice"}, nil
}

/*
answered is the path an answer to buttons takes, false when it named
none and the step takes no typed answer.
*/
func answered(s workflowspec.Step, answer DryAnswer, b workflowspec.ButtonsSettings) (outcome, bool) {
	paths, typing := buttonPaths(s), workflowspec.On(b.AllowTyping, true)
	switch {
	case answer.Silent:
		return withDetail(fixedOrEnd(s, workflowspec.OutcomeTimeout, EndNoChoice), "answer", "none"), true
	case answer.Choice >= 1 && answer.Choice <= len(paths):
		p := paths[answer.Choice-1]
		return outcome{path: p.ID, outcome: "button", detail: map[string]any{"answer": p.Label}}, true
	}
	if p, ok := matchChoice(paths, answer.Text); ok {
		return outcome{path: p.ID, outcome: "button", detail: map[string]any{"answer": answer.Text}}, true
	}
	if typed, ok := workflowspec.PathFor(s, workflowspec.OutcomeTyped); ok && typing {
		typedOut := outcome{path: typed.ID, outcome: workflowspec.OutcomeTyped}
		return withDetail(typedOut, "answer", answer.Text), true
	}

	return outcome{}, false
}

/*
collect asks for one value, unless it is known, and saves a valid
answer in the dry run's copy; a wrong one is asked again up to the
step's attempts.
*/
func (d *dry) collect(ctx context.Context, s workflowspec.Step) (outcome, error) {
	c := settings[workflowspec.CollectSettings](s)
	if workflowspec.On(c.SkipIfKnown, true) && d.known(c.Field) {
		return outcome{outcome: outcomeAlreadyKnown}, nil
	}
	text := d.render(ctx, c.Text)
	var options []string
	if c.Type == "choice" || c.Type == "yes_no" {
		options = choices(c)
	}
	d.in.Driver.Said(ctx, DryMessage{StepID: s.ID, Text: text, Choices: options})
	limit := max(c.MaxAttempts, 0)
	if limit == 0 {
		limit = defaultAttempts
	}
	ask := DryAsk{StepID: s.ID, Kind: s.Kind, Question: text, Choices: options, CanWait: c.Timeout != "" || c.AllowSkip}
	for attempt := 1; ; attempt++ {
		answer, err := d.in.Driver.Customer(ctx, ask)
		if err != nil {
			return outcome{}, err
		}
		if out, done := d.collected(s, c, answer, options); done {
			return out, nil
		}
		if attempt >= limit {
			out := fixedOrNext(s, workflowspec.OutcomeInvalid)
			out.outcome = outcomeInvalidAnswer
			return out, nil
		}
		retry := cmpText(c.InvalidText, "Sorry, that doesn't look right. Could you try again?")
		d.in.Driver.Said(ctx, DryMessage{StepID: s.ID, Text: retry})
	}
}

/*
collected saves a valid answer and says where the step goes, false for
an answer to ask again.
*/
func (d *dry) collected(s workflowspec.Step, c workflowspec.CollectSettings, answer DryAnswer, options []string,
) (outcome, bool) {
	if answer.Silent {
		return withDetail(fixedOrNext(s, workflowspec.OutcomeTimeout), "answer", "none"), true
	}
	raw := answer.Text
	if answer.Choice >= 1 && answer.Choice <= len(options) {
		raw = options[answer.Choice-1]
	}
	value, ok := parseAnswer(c, raw)
	if !ok {
		return outcome{}, false
	}
	d.store(c.Field, value)

	detail := map[string]any{"field": c.Field, "value": value}
	return outcome{outcome: outcomeCollected, acted: true, detail: detail}, true
}

/*
withDetail notes one detail on an outcome.
*/
func withDetail(out outcome, key string, value any) outcome {
	if out.detail == nil {
		out.detail = map[string]any{}
	}
	out.detail[key] = value

	return out
}

/*
known reports whether a field already has a value.
*/
func (d *dry) known(field string) bool {
	if key, ok := strings.CutPrefix(field, "run.vars."); ok {
		return d.out.Vars[key] != nil
	}
	d.refresh()
	_, ok := d.subj.Lookup(field)

	return ok
}

/*
store saves a value in the dry run's copy: a run variable, or the
contact's field, with an email the contact already has kept.
*/
func (d *dry) store(field string, value any) {
	if key, ok := strings.CutPrefix(field, "run.vars."); ok {
		d.out.Vars[key] = value
		return
	}
	params, ok := contactChange(d.subj.contact, field, value)
	if !ok {
		return
	}
	c := &d.subj.contact
	c.Email, c.Name, c.Phone, c.CustomAttributes = params.Email, params.Name, params.Phone, params.CustomAttributes
}

/*
branch takes the first path whose condition holds, as a run does.
*/
func (d *dry) branch(ctx context.Context, s workflowspec.Step) (outcome, error) {
	d.refresh()
	failed := map[string]string{}
	var taken *outcome
	err := d.read(ctx, func(ctx context.Context, q *models.Queries) error {
		for _, p := range buttonPaths(s) {
			held, field, err := d.holds(ctx, q, p)
			if err != nil || held {
				if held {
					taken = &outcome{path: p.ID, outcome: outcomeMatched, detail: map[string]any{"failed": failed}}
				}
				return err
			}
			if field != "" {
				failed[p.ID] = field
			}
		}
		return nil
	})
	if err != nil || taken != nil {
		return deref2(taken), err
	}

	return withDetail(fixedOrNext(s, workflowspec.OutcomeElse), "failed", failed), nil
}

/*
holds reports whether a branch path's condition holds, and the first
field that failed when it does not. A condition that does not read
never holds.
*/
func (d *dry) holds(ctx context.Context, q *models.Queries, p workflowspec.Path) (bool, string, error) {
	rule, err := audience.ParseRule(p.Condition)
	if err != nil {
		return false, "", nil //nolint:nilerr // a condition that does not read never holds, as in a run
	}
	program, err := audience.CompileRule(rule, workflowspec.Catalog(), audience.WorkflowBranch())
	if err != nil {
		return false, "", nil //nolint:nilerr // a condition that does not compile never holds, as in a run
	}
	if err := d.subj.withSaved(ctx, q, program); err != nil {
		return false, "", err
	}
	explained := program.Explain(d.subj.env())
	if explained.Result == audience.True {
		return true, "", nil
	}
	leaf, _ := explained.FirstFailing()

	return false, leaf.Field, nil
}

/*
deref2 returns the outcome, none for nil.
*/
func deref2(o *outcome) outcome {
	if o == nil {
		return outcome{}
	}

	return *o
}

/*
classify takes the category the step's mock names, or the one the
classifier picks from what the customer wrote.
*/
func (d *dry) classify(ctx context.Context, s workflowspec.Step) outcome {
	c := settings[workflowspec.ClassifySettings](s)
	cats := categories(s)
	answer, reason := d.sorted(ctx, s, c, cats)
	if reason != "" {
		return unsorted(s, reason)
	}
	label, ok := categoryLabel(cats, answer.CategoryID)
	if !ok {
		return unsorted(s, "no_match")
	}
	if !sureEnough(answer.Confidence, c.MinConfidence) {
		return withDetail(unsorted(s, "low_confidence"), "category", label)
	}
	if c.SaveTo != "" {
		d.store(c.SaveTo, label)
	}

	return outcome{path: answer.CategoryID, outcome: outcomeMatched, detail: map[string]any{
		"category": label, "confidence": answer.Confidence,
	}}
}

/*
sorted is the category a classify step's mock names, or the one the
classifier picks, with why there is none when there is not.
*/
func (d *dry) sorted(ctx context.Context, s workflowspec.Step, c workflowspec.ClassifySettings, cats []Category,
) (ClassifyAnswer, string) {
	if mock, ok := d.in.Mocks[s.ID]; ok {
		var picked struct {
			PathID string `json:"path_id"`
		}
		_ = json.Unmarshal(mock.Body, &picked)
		return ClassifyAnswer{CategoryID: picked.PathID, Confidence: "high"}, ""
	}
	if d.e.Classifier == nil {
		return ClassifyAnswer{}, "ai_unavailable"
	}
	said := d.in.Driver.Words()
	if c.Source == "last_message" && len(said) > 0 {
		said = said[len(said)-1:]
	}
	text := strings.Join(said, "\n\n")
	if strings.TrimSpace(text) == "" {
		return ClassifyAnswer{}, "no_text"
	}
	got, err := d.e.Classifier.Classify(ctx, ClassifyQuestion{
		WorkspaceID: d.in.WorkspaceID, ConversationID: d.conv.ID, Instructions: c.Question, Categories: cats,
		Text: text,
	})
	if err != nil {
		return ClassifyAnswer{}, "ai_unavailable"
	}
	return got, ""
}

/*
handle lets dimmi handle the conversation: the driver plays the agent's
turns, and the step takes the path of how they ended.
*/
func (d *dry) handle(ctx context.Context, s workflowspec.Step) (outcome, error) {
	h := DryHandle{StepID: s.ID}
	if public := settings[workflowspec.DimmiSettings](s).AgentID; public != "" {
		if id, ok := d.decode(models.PrefixAiAgent, public); ok {
			h.AgentID = &id
		}
	}
	got, err := d.in.Driver.Handle(ctx, h)
	if err != nil {
		return outcome{}, err
	}
	fixed := got.Outcome
	if !slices.Contains([]string{
		workflowspec.OutcomeResolved, workflowspec.OutcomeHandedOver,
		workflowspec.OutcomeQuiet,
	}, fixed) {
		fixed = workflowspec.OutcomeQuiet
	}
	out := fixedOrEnd(s, fixed, EndCompleted)
	if fixed == workflowspec.OutcomeHandedOver && out.end != "" {
		out.end = DryHandedOver
	}

	return withDetail(out, "reason", got.Reason), nil
}

/*
request answers an HTTP request step with its mock, or with success and
nothing when it has none: no request leaves dimmi. A failing status
takes the failure path; values the step saves are read from the
mock's body.
*/
func (d *dry) request(s workflowspec.Step) outcome {
	r := settings[workflowspec.HTTPRequestSettings](s)
	method := r.Method
	if method == "" {
		method = "GET"
	}
	summary := "Would call " + method + " " + cmpText(r.URL, "the connection's endpoint")
	mock, mocked := d.in.Mocks[s.ID]
	status := 200
	if mocked && mock.Status != 0 {
		status = mock.Status
	}
	d.out.Actions = append(d.out.Actions, DryAction{
		StepID: s.ID, Kind: s.Kind, Summary: summary, Mocked: mocked,
		Detail: map[string]any{"method": method, "url": r.URL, "status": status},
	})
	if status >= 400 {
		return withDetail(fixedOrNext(s, workflowspec.OutcomeFailure), "status", status)
	}
	var body any
	if mocked && len(mock.Body) > 0 {
		_ = json.Unmarshal(mock.Body, &body)
	}
	for _, save := range r.Save {
		if v := pathValue(body, save.Path); v != nil {
			d.store(save.Field, v)
		}
	}

	return outcome{outcome: "succeeded", acted: true, detail: map[string]any{"status": status, "mocked": mocked}}
}

/*
pathValue reads a plain dotted path from a decoded body.
*/
func pathValue(body any, path string) any {
	at := body
	for key := range strings.SplitSeq(path, ".") {
		m, ok := at.(map[string]any)
		if !ok {
			return nil
		}
		at = m[key]
	}

	return at
}

/*
cmpText is the text, or the fallback when it is empty.
*/
func cmpText(text, fallback string) string {
	if strings.TrimSpace(text) == "" {
		return fallback
	}

	return text
}
