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
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/enums"
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
	paths := buttonPaths(s)
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
		if answer.Silent {
			return withDetail(fixedOrEnd(s, workflowspec.OutcomeTimeout, EndNoChoice), "answer", "none"), nil
		}
		if answer.Choice >= 1 && answer.Choice <= len(paths) {
			p := paths[answer.Choice-1]
			return outcome{path: p.ID, outcome: "button", detail: map[string]any{"answer": p.Label}}, nil
		}
		if p, ok := matchChoice(paths, answer.Text); ok {
			return outcome{path: p.ID, outcome: "button", detail: map[string]any{"answer": answer.Text}}, nil
		}
		if typed, ok := workflowspec.PathFor(s, workflowspec.OutcomeTyped); ok && typing {
			return outcome{path: typed.ID, outcome: workflowspec.OutcomeTyped,
				detail: map[string]any{"answer": answer.Text}}, nil
		}
		if try == 0 {
			retry := b.RetryText
			if retry == "" {
				retry = "Please reply with one of the numbers above."
			}
			d.in.Driver.Said(ctx, DryMessage{StepID: s.ID, Text: retry, Choices: labels})
		}
	}

	return outcome{end: EndNoChoice, outcome: "no_choice"}, nil
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
	limit := c.MaxAttempts
	if limit == 0 {
		limit = defaultAttempts
	}
	for attempt := 1; ; attempt++ {
		answer, err := d.in.Driver.Customer(ctx, DryAsk{
			StepID: s.ID, Kind: s.Kind, Question: text, Choices: options, CanWait: c.Timeout != "" || c.AllowSkip,
		})
		if err != nil {
			return outcome{}, err
		}
		if answer.Silent {
			return withDetail(fixedOrNext(s, workflowspec.OutcomeTimeout), "answer", "none"), nil
		}
		raw := answer.Text
		if answer.Choice >= 1 && answer.Choice <= len(options) {
			raw = options[answer.Choice-1]
		}
		if value, ok := parseAnswer(c, raw); ok {
			d.store(c.Field, value)
			return outcome{outcome: outcomeCollected, acted: true, detail: map[string]any{
				"field": c.Field, "value": value,
			}}, nil
		}
		if attempt >= limit {
			out := fixedOrNext(s, workflowspec.OutcomeInvalid)
			out.outcome = outcomeInvalidAnswer
			return out, nil
		}
		retry := c.InvalidText
		if retry == "" {
			retry = "Sorry, that doesn't look right. Could you try again?"
		}
		d.in.Driver.Said(ctx, DryMessage{StepID: s.ID, Text: retry})
	}
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
			rule, err := audience.ParseRule(p.Condition)
			if err != nil {
				continue
			}
			program, err := audience.CompileRule(rule, workflowspec.Catalog(), audience.WorkflowBranch())
			if err != nil {
				continue
			}
			if err := d.subj.withSaved(ctx, q, program); err != nil {
				return err
			}
			explained := program.Explain(d.subj.env())
			if explained.Result == audience.True {
				taken = &outcome{path: p.ID, outcome: outcomeMatched, detail: map[string]any{"failed": failed}}
				return nil
			}
			if leaf, ok := explained.FirstFailing(); ok {
				failed[p.ID] = leaf.Field
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
	answer := ClassifyAnswer{}
	if mock, ok := d.in.Mocks[s.ID]; ok {
		var picked struct {
			PathID string `json:"path_id"`
		}
		_ = json.Unmarshal(mock.Body, &picked)
		answer.CategoryID, answer.Confidence = picked.PathID, "high"
	} else {
		if d.e.Classifier == nil {
			return unsorted(s, "ai_unavailable")
		}
		said := d.in.Driver.Words()
		if c.Source == "last_message" && len(said) > 0 {
			said = said[len(said)-1:]
		}
		text := strings.Join(said, "\n\n")
		if strings.TrimSpace(text) == "" {
			return unsorted(s, "no_text")
		}
		got, err := d.e.Classifier.Classify(ctx, ClassifyQuestion{
			WorkspaceID: d.in.WorkspaceID, ConversationID: d.conv.ID, Instructions: c.Question, Categories: cats,
			Text: text,
		})
		if err != nil {
			return unsorted(s, "ai_unavailable")
		}
		answer = got
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
	if !slices.Contains([]string{workflowspec.OutcomeResolved, workflowspec.OutcomeHandedOver,
		workflowspec.OutcomeQuiet}, fixed) {
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
	for _, key := range strings.Split(path, ".") {
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

/*
action records what an action would do and changes the dry run's copy
of the conversation where a later step could read it.
*/
func (d *dry) action(ctx context.Context, s workflowspec.Step) outcome {
	switch s.Kind {
	case workflowspec.KindTag, workflowspec.KindUntag:
		tags := settings[workflowspec.TagSettings](s).TagIDs
		if s.Kind == workflowspec.KindTag {
			for _, t := range tags {
				if !slices.Contains(d.subj.conversationTags, t) {
					d.subj.conversationTags = append(d.subj.conversationTags, t)
				}
			}
			return d.act(s, "Would add tags", map[string]any{"tag_ids": tags})
		}
		d.subj.conversationTags = slices.DeleteFunc(d.subj.conversationTags, func(t string) bool {
			return slices.Contains(tags, t)
		})
		return d.act(s, "Would remove tags", map[string]any{"tag_ids": tags})
	case workflowspec.KindPriority:
		value := settings[workflowspec.PrioritySettings](s).Value
		if level, ok := enums.ConversationPriority(value).Level(); ok {
			d.conv.Priority = level
		}
		return d.act(s, "Would set the priority to "+value, map[string]any{"priority": value})
	case workflowspec.KindAssign:
		return d.assign(s)
	case workflowspec.KindUnassign:
		what := settings[workflowspec.UnassignSettings](s).What
		if what != "team" {
			d.conv.AssigneeMemberID = nil
		}
		if what != "member" {
			d.conv.AssigneeTeamID = nil
		}
		return d.act(s, "Would unassign the conversation", map[string]any{"what": what})
	case workflowspec.KindNote:
		text := d.render(ctx, settings[workflowspec.NoteSettings](s).Text)
		return d.act(s, "Would leave a note", map[string]any{"text": text})
	case workflowspec.KindUpdateContact:
		var set []map[string]any
		for _, c := range settings[workflowspec.UpdateContactSettings](s).Set {
			value := d.render(ctx, c.Value)
			d.store(c.Field, value)
			set = append(set, map[string]any{"field": c.Field, "value": value})
		}
		return d.act(s, "Would update the contact", map[string]any{"set": set})
	case workflowspec.KindSnooze:
		d.conv.Status = "snoozed"
		return d.act(s, "Would snooze the conversation", nil)
	case workflowspec.KindClose:
		d.conv.Status = "closed"
		return d.act(s, "Would close the conversation", nil)
	case workflowspec.KindApplySLA:
		id := settings[workflowspec.ApplySLASettings](s).SLAPolicyID
		return d.act(s, "Would apply an SLA policy", map[string]any{"sla_policy_id": id})
	case workflowspec.KindRunWorkflow:
		id := settings[workflowspec.RunWorkflowSettings](s).WorkflowID
		return d.act(s, "Would run another workflow", map[string]any{"workflow_id": id})
	}
	if k, ok := workflowspec.FindKind(s.Kind); ok && k.Action {
		var detail map[string]any
		_ = json.Unmarshal(s.Settings, &detail)
		return d.act(s, "Would run "+strings.ReplaceAll(s.Kind, "_", " "), detail)
	}

	return outcome{outcome: "not_simulated", status: "skipped"}
}

/*
assign records an assignment and makes it in the dry run's copy.
*/
func (d *dry) assign(s workflowspec.Step) outcome {
	a := settings[workflowspec.AssignSettings](s)
	if workflowspec.On(a.KeepIfAssigned, true) && d.conv.AssigneeMemberID != nil {
		return outcome{outcome: outcomeKept, acted: true}
	}
	detail := map[string]any{"mode": a.Mode}
	if a.Mode == "member" {
		if id, ok := d.decode(models.PrefixMember, a.MemberID); ok {
			d.conv.AssigneeMemberID = &id
		}
		detail["member_id"] = a.MemberID
		return d.act(s, "Would assign to a teammate", detail)
	}
	if id, ok := d.decode(models.PrefixTeam, a.TeamID); ok {
		d.conv.AssigneeTeamID = &id
	}
	detail["team_id"] = a.TeamID
	summary := "Would assign to a team"
	if a.Mode != "team" {
		summary = fmt.Sprintf("Would assign to a teammate of the team (%s)", strings.ReplaceAll(a.Mode, "_", " "))
	}

	return d.act(s, summary, detail)
}
