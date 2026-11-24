/*
|--------------------------------------------------------------------------
| Extracting Values And Running Actions
|--------------------------------------------------------------------------
|
| An extract step and a run action step leave the database like an AI
| path: the job makes the call and keeps what it found on the wait, and
| the move that follows writes it to the person in the journey's own
| transaction, so it is written exactly once. The AI reads the person's
| profile and what they did lately; an action reads the details of the
| person its inputs name.
|
*/

package series

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/eventroll"
)

/*
ActionRunner runs the workspace's actions for automations.
*/
type ActionRunner interface {
	/*
		RunForAutomation runs an action's published version.
	*/
	RunForAutomation(ctx context.Context, in actionkit.AutomationCall) (actionkit.AutomationResult, error)
}

/*
The results a call keeps on the wait, and how much of what a person
did the AI reads.
*/
const (
	resultOK       = "ok"
	resultFailed   = "failed:"
	maxEventsShown = 20
	maxPeopleRead  = 10
)

/*
profileWithEvents is what the AI reads about the traveler: their
profile, then the events they did most lately.
*/
func profileWithEvents(ctx context.Context, r *run) (string, error) {
	people := r.people[:min(len(r.people), maxPeopleRead)]
	if len(people) == 0 {
		return profileOf(r), nil
	}

	contacts := make([]ids.UUID, 0, len(people))
	for _, c := range people {
		contacts = append(contacts, c.ID)
	}

	events, err := audiences.LoadEvents(ctx, r.q, r.e.WorkspaceID, contacts)
	if err != nil {
		return "", fmt.Errorf("load events: %w", err)
	}

	var b strings.Builder

	b.WriteString(profileOf(r))

	for _, c := range people {
		if lines := eventLines(events[c.ID]); len(lines) > 0 {
			fmt.Fprintf(&b, "Recent events of %s:\n%s\n", deref(c.Name), strings.Join(lines, "\n"))
		}
	}

	return b.String(), nil
}

/*
eventLines lists a person's events, the most recent first.
*/
func eventLines(events map[string]eventroll.Summary) []string {
	names := make([]string, 0, len(events))
	for name := range events {
		names = append(names, name)
	}

	slices.SortFunc(names, func(a, b string) int { return events[b].LastAt.Compare(events[a].LastAt) })

	lines := make([]string, 0, min(len(names), maxEventsShown))
	for _, name := range names[:min(len(names), maxEventsShown)] {
		e := events[name]
		lines = append(lines, fmt.Sprintf("- %s: %d times, last on %s", name, e.Count, e.LastAt.Format(time.DateOnly)))
	}

	return lines
}

/*
extract asks the AI for the values the step looks for.
*/
func (s *Series) extract(ctx context.Context, c *stepCall) (string, map[string]json.RawMessage) {
	if s.Decisions == nil {
		return "ai_unavailable", nil
	}

	settings := settingsOf[ExtractSettings](c.n)

	fields := make([]aidecision.Field, 0, len(settings.Fields))
	for _, f := range settings.Fields {
		fields = append(fields, aidecision.Field{
			Key: f.Key, Label: f.Label, Description: f.Description, Type: f.Type, Options: f.Options,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(s.Config.AITimeoutSeconds, 1))*time.Second)
	defer cancel()

	got, err := s.Decisions.Extract(ctx, aidecision.Extraction{
		Subject:      aidecision.Subject{WorkspaceID: c.r.e.WorkspaceID, Context: c.profile},
		Instructions: settings.Prompt, Fields: fields,
	})
	if err != nil {
		return "ai_unavailable", nil
	}

	values := map[string]json.RawMessage{}
	for key, v := range got.Values {
		if raw, err := json.Marshal(v); err == nil {
			values[key] = raw
		}
	}

	return resultOK, values
}

/*
extracted writes what the AI found, and the fallbacks of what it did
not, to the person. A value with no fallback that stayed unknown takes
the fallback path, or goes on when that leads nowhere.
*/
func (r *run) extracted(ctx context.Context, n Node, w wait) (outcome, error) {
	unsure := 0

	for _, f := range settingsOf[ExtractSettings](n).Fields {
		raw, ok := w.Values[f.Key]
		if !ok || !fits(f, raw) {
			raw, ok = f.Fallback, len(f.Fallback) > 0 && string(f.Fallback) != "null"
		}

		if !ok {
			unsure++
			continue
		}

		if err := r.attribute(ctx, f.Key, raw); err != nil {
			return outcome{}, err
		}
	}

	if err := r.recheck(ctx); err != nil {
		return outcome{}, err
	}

	if unsure > 0 && r.g.hasPath(n.ID, PathFallback) {
		return outcome{path: PathFallback, result: "extract_fallback"}, nil
	}

	return outcome{path: PathNext, result: "extract:" + cmp.Or(w.Result, "timed_out")}, nil
}

/*
runAction runs the step's action for the person and keeps the outputs
it writes.
*/
func (s *Series) runAction(ctx context.Context, c *stepCall) (string, map[string]json.RawMessage) {
	if s.Actions == nil || c.action == nil {
		return resultFailed + "unavailable", nil
	}

	res, err := s.Actions.RunForAutomation(tenancy.With(ctx, c.action.WorkspaceID.String()), *c.action)
	if err != nil {
		return resultFailed + "unavailable", nil
	}

	if !res.Succeeded {
		return resultFailed + cmp.Or(res.ErrorCode, "failed"), nil
	}

	values := map[string]json.RawMessage{}

	var result map[string]json.RawMessage
	if json.Unmarshal(res.Result, &result) == nil {
		for _, sv := range settingsOf[RunActionSettings](c.n).Save {
			if raw, ok := result[sv.Output]; ok {
				values[sv.Attribute] = raw
			}
		}
	}

	return resultOK, values
}

/*
acted writes the action's outputs to the person and goes on, or takes
the failed path when the call failed.
*/
func (r *run) acted(ctx context.Context, n Node, w wait) (outcome, error) {
	if w.Result != resultOK {
		path := PathNext
		if r.g.hasPath(n.ID, PathFailed) {
			path = PathFailed
		}

		return outcome{path: path, result: "action:" + cut(cmp.Or(strings.TrimPrefix(w.Result, resultFailed),
			"timed_out"), 30)}, nil
	}

	keys := make([]string, 0, len(w.Values))
	for key := range w.Values {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for _, key := range keys {
		if err := r.attribute(ctx, key, w.Values[key]); err != nil {
			return outcome{}, err
		}
	}

	if len(keys) > 0 {
		if err := r.recheck(ctx); err != nil {
			return outcome{}, err
		}
	}

	return outcome{path: PathNext, result: "action:ok"}, nil
}

/*
prepareAction reads the person the step's inputs name and fills them.
*/
func (s *Series) prepareAction(ctx context.Context, r *run, n Node) *actionkit.AutomationCall {
	settings := settingsOf[RunActionSettings](n)

	action, err := s.IDs.Decode(string(models.PrefixAiAction), settings.ActionID)
	if err != nil {
		return nil
	}

	in := &actionkit.AutomationCall{
		WorkspaceID: r.e.WorkspaceID, ActionID: action, Inputs: map[string]string{}, By: "Series",
	}

	var person *models.Contact
	if len(r.people) > 0 {
		person = &r.people[0]
		in.ContactID, in.Identity = &person.ID, actionkit.AutomationIdentity(*person)
	}

	var company *models.Company

	if r.company != nil {
		c, err := r.q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: r.e.WorkspaceID, ID: *r.company})
		if err == nil {
			company = &c
		}
	}

	for _, input := range settings.Inputs {
		in.Inputs[input.Name] = fillDetails(input.Value, person, company)
	}

	return in
}

/*
detailPattern matches a detail of the person an input names.
*/
var detailPattern = regexp.MustCompile(`\{\{\s*([a-z_]+(?:\.[a-z0-9_]+){1,2})\s*\}\}`)

/*
fillDetails fills the details of the person and their company an input
names, empty for any they do not have.
*/
func fillDetails(text string, person *models.Contact, company *models.Company) string {
	return detailPattern.ReplaceAllStringFunc(text, func(found string) string {
		name := detailPattern.FindStringSubmatch(found)[1]

		return detailOf(name, person, company)
	})
}

/*
detailOf reads one detail of the person or their company.
*/
func detailOf(name string, person *models.Contact, company *models.Company) string {
	root, rest, _ := strings.Cut(name, ".")

	switch {
	case root == "contact" && person != nil:
		switch rest {
		case "email":
			return deref(person.Email)
		case "name":
			return deref(person.Name)
		case "first_name":
			first, _, _ := strings.Cut(deref(person.Name), " ")
			return first
		case "phone":
			return deref(person.Phone)
		case "external_id":
			return deref(person.ExternalID)
		case "id":
			return models.PublicID(models.PrefixContact, person.ID)
		}

		if key, ok := strings.CutPrefix(rest, "attr."); ok {
			return attributeText(person.CustomAttributes, key)
		}
	case root == "company" && company != nil:
		switch rest {
		case "name":
			return company.Name
		case "id":
			return models.PublicID(models.PrefixCompany, company.ID)
		}

		if key, ok := strings.CutPrefix(rest, "attr."); ok {
			return attributeText(company.CustomAttributes, key)
		}
	}

	return ""
}

/*
attributeText reads one custom attribute as text.
*/
func attributeText(raw json.RawMessage, key string) string {
	var all map[string]any
	if json.Unmarshal(raw, &all) != nil || all[key] == nil {
		return ""
	}

	if s, ok := all[key].(string); ok {
		return s
	}

	out, _ := json.Marshal(all[key])

	return string(out)
}
