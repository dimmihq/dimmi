/*
|--------------------------------------------------------------------------
| Dry Run Articles, Reply Times, Extracts And Actions
|--------------------------------------------------------------------------
|
| An article step reads the article it names, and says its title; one
| that is gone is skipped, as in a run. A reply time step works out the
| reply time the way a run does, reading the office hours and how fast
| the team answered lately, and writes nothing.
|
| An extract step takes the values its mock names, keyed by step id, or
| asks the decision models about what the customer wrote, then saves
| them, or their fallbacks, in the dry run's copy so later steps read
| them. A value with no fallback that stays unknown takes the none path.
|
| A run action step never runs the action. Its mock, by step id or by
| the action's public id, answers in its place: a failing status takes
| the failure path, and the outputs the step saves are read from the
| mock's body. Without a mock it succeeds with nothing.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
article says the step's line and the article's title.
*/
func (d *dry) article(ctx context.Context, s workflowspec.Step) (outcome, error) {
	a := settings[workflowspec.ArticleSettings](s)
	id, ok := d.decode(models.PrefixArticle, a.Article)
	if !ok {
		return skipped(outcomeArticleGone), nil
	}
	var title string
	found := false
	err := d.read(ctx, func(ctx context.Context, q *models.Queries) error {
		row, err := q.FindArticle(ctx, models.FindArticleParams{WorkspaceID: d.in.WorkspaceID, ID: id})
		title, found = row.Title, err == nil
		return nil
	})
	if err != nil || !found {
		return skipped(outcomeArticleGone), err
	}
	out := d.say(ctx, s, strings.TrimSpace(d.render(ctx, a.Text)+"\n\n[Help article: "+title+"]"))

	return withDetail(withDetail(out, "article_id", a.Article), "title", title), nil
}

/*
replyTime says when the team replies, worked out as a run does.
*/
func (d *dry) replyTime(ctx context.Context, s workflowspec.Step) (outcome, error) {
	r := settings[workflowspec.ReplyTimeSettings](s)
	var phrase string
	var open bool
	err := d.read(ctx, func(ctx context.Context, q *models.Queries) error {
		var err error
		phrase, open, err = d.replyPhrase(ctx, q, r)
		return err
	})
	if err != nil {
		return outcome{}, err
	}
	text := cmpText(r.Text, workflowspec.DefaultReplyTimeText)
	out := d.say(ctx, s, replyTimeVariable().ReplaceAllLiteralString(text, phrase))

	return withDetail(withDetail(out, "reply_time", phrase), "open", open), nil
}

/*
replyPhrase is the reply time in words and whether the team is open:
when it is back outside office hours, how fast it answers inside them.
*/
func (d *dry) replyPhrase(ctx context.Context, q *models.Queries, r workflowspec.ReplyTimeSettings,
) (string, bool, error) {
	now := d.e.Clock.Now()
	sched, err := d.e.hours(ctx, q, placeOf(d.conv), r.ScheduleID)
	if err != nil && !scheduleProblem(err) {
		return "", false, err
	}
	if err == nil && !sched.IsOpen(now) {
		if next, err := sched.NextOpen(now); err == nil {
			return sched.BackAt(now, next), false, nil
		}
	}
	params := models.ReplyTimeFactsParams{WorkspaceID: d.in.WorkspaceID, Since: now.Add(-replyHistory)}
	if team, ok := d.decode(models.PrefixTeam, r.TeamID); ok {
		params.TeamID = &team
	}
	facts, err := q.ReplyTimeFacts(ctx, params)
	if err != nil {
		return "", true, err
	}

	return withinWords(estimate(facts)), true, nil
}

/*
extract saves the values the step's mock names, or the ones the
decision models find in what the customer wrote, and the fallbacks of
the others.
*/
func (d *dry) extract(ctx context.Context, s workflowspec.Step) outcome {
	e := settings[workflowspec.ExtractSettings](s)
	answer := d.extraction(ctx, s, e)

	var saved, unsure []string
	values := map[string]any{}
	for _, f := range e.Fields {
		value, ok := answer.Values[f.Key]
		if !ok || !workflowspec.FitsValue(f, mustJSON(value)) {
			value, ok = fallbackOf(f)
		}
		if !ok {
			unsure = append(unsure, f.Key)
			continue
		}
		d.store(f.Target(), value)
		saved, values[f.Target()] = append(saved, f.Target()), value
	}
	out := extracted(s, saved, unsure, answer.Reason)
	out = withDetail(out, "values", values)
	if _, mocked := d.in.Mocks[s.ID]; mocked {
		out = withDetail(out, "mocked", true)
	}

	return out
}

/*
extraction is what the step's mock names, or what the decision models
find in what the customer wrote.
*/
func (d *dry) extraction(ctx context.Context, s workflowspec.Step, e workflowspec.ExtractSettings) callAnswer {
	if mock, ok := d.in.Mocks[s.ID]; ok {
		values := map[string]any{}
		_ = json.Unmarshal(mock.Body, &values)
		return callAnswer{Values: values}
	}
	if d.e.Decisions == nil {
		return callAnswer{Reason: "ai_unavailable"}
	}
	said := d.in.Driver.Words()
	if e.Source == "last_message" && len(said) > 0 {
		said = said[len(said)-1:]
	}
	text := strings.Join(said, "\n\n")
	if strings.TrimSpace(text) == "" {
		return callAnswer{Reason: "no_text"}
	}
	fields := make([]aidecision.Field, 0, len(e.Fields))
	for _, f := range e.Fields {
		fields = append(fields, aidecision.Field{
			Key: f.Key, Label: f.Label, Description: f.Description, Type: f.Type, Options: f.Options,
		})
	}
	return *d.e.extract(ctx, aidecision.Extraction{
		Subject: aidecision.Subject{
			WorkspaceID: d.in.WorkspaceID, ConversationID: d.conversation(), Context: text,
		},
		Instructions: e.Instructions, Fields: fields,
	})
}

/*
runAction answers a run action step with its mock, never the action:
a failing status takes the failure path, and the outputs it saves are
read from the mock's body.
*/
func (d *dry) runAction(ctx context.Context, s workflowspec.Step) outcome {
	a := settings[workflowspec.RunActionSettings](s)
	inputs := map[string]any{}
	for _, in := range a.Inputs {
		inputs[in.Name] = d.render(ctx, in.Value)
	}
	mock, mocked := d.in.Mocks[s.ID]
	if !mocked {
		mock, mocked = d.in.Mocks[a.ActionID]
	}
	status := 200
	if mocked && mock.Status != 0 {
		status = mock.Status
	}
	d.out.Actions = append(d.out.Actions, DryAction{
		StepID: s.ID, Kind: s.Kind, Summary: "Would run the action " + d.actionName(ctx, a.ActionID), Mocked: mocked,
		Detail: map[string]any{"action_id": a.ActionID, "inputs": inputs, "status": status},
	})
	if status < 200 || status >= 300 {
		return requestFailed(s, "failed", status)
	}
	saved := map[string]any{}
	if mocked {
		for _, sv := range outputValues(a.Save, mock.Body) {
			d.store(sv.Field, sv.Value)
			saved[sv.Field] = sv.Value
		}
	}

	return outcome{outcome: "succeeded", acted: true, detail: map[string]any{
		"status": status, "mocked": mocked, "saved": saved,
	}}
}

/*
actionName is the title of the action a step runs, its id when it
cannot be read.
*/
func (d *dry) actionName(ctx context.Context, public string) string {
	name := public
	id, ok := d.decode(models.PrefixAiAction, public)
	if !ok {
		return name
	}
	_ = d.read(ctx, func(ctx context.Context, q *models.Queries) error {
		row, err := q.FindAction(ctx, models.FindActionParams{WorkspaceID: d.in.WorkspaceID, ID: id})
		if err == nil {
			name = cmpText(row.Title, row.Name)
		}
		return nil
	})

	return name
}
