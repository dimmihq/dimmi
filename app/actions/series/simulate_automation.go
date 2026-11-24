/*
|--------------------------------------------------------------------------
| Dry Simulation Extracts, Actions And Tags
|--------------------------------------------------------------------------
|
| An extract step takes the values its mock names, keyed by step id,
| or asks the AI a journey asks about the person's profile; the values,
| or their fallbacks, are kept as the person's attributes for the steps
| after it, and a value that stays unknown takes the fallback path when
| there is one. A run action step never runs the action: its mock, by
| step id or by the action's public id, answers in its place, a failing
| status takes the failed path, and the outputs it saves are kept as
| attributes. Tags the simulation would add or remove are kept too, so
| a later rule about a tag reads them.
|
*/

package series

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
)

/*
adjust gives a person the tags the simulation would have added or
removed before a rule reads them.
*/
func (r *simRun) adjust(p *audiences.Person) {
	for tag, on := range r.tags {
		has := slices.Contains(p.ContactTags, tag)
		switch {
		case on && !has:
			p.ContactTags = append(p.ContactTags, tag)
		case !on && has:
			p.ContactTags = slices.DeleteFunc(p.ContactTags, func(t string) bool { return t == tag })
		}
	}
}

/*
setAttribute keeps one attribute of the person in memory.
*/
func (r *simRun) setAttribute(key string, value any) {
	attrs := map[string]any{}
	_ = json.Unmarshal(r.person.CustomAttributes, &attrs)
	attrs[key] = value
	r.person.CustomAttributes, _ = json.Marshal(attrs)
}

/*
extract keeps the values the step's mock names, or the AI finds, and
the fallbacks of the others.
*/
func (r *simRun) extract(ctx context.Context, n Node) outcome {
	_, mocked := r.in.Mocks[n.ID]
	result, values := r.extraction(ctx, n)
	written, unsure := map[string]any{}, []string{}
	for _, f := range settingsOf[ExtractSettings](n).Fields {
		raw, ok := values[f.Key]
		if !ok || !fits(f, raw) {
			raw, ok = f.Fallback, len(f.Fallback) > 0 && string(f.Fallback) != "null"
		}
		if !ok {
			unsure = append(unsure, f.Key)
			continue
		}
		var value any
		_ = json.Unmarshal(raw, &value)
		r.setAttribute(f.Key, value)
		written[f.Key] = value
	}
	details := map[string]any{"values": written, "unsure": unsure, "mocked": mocked}
	if len(unsure) > 0 && r.g.hasPath(n.ID, PathFallback) {
		return outcome{path: PathFallback, result: "extract_fallback", details: details}
	}

	return outcome{path: PathNext, result: "extract:" + result, details: details}
}

/*
extraction is what the step's mock names, or what the AI finds in the
person's profile.
*/
func (r *simRun) extraction(ctx context.Context, n Node) (string, map[string]json.RawMessage) {
	if mock, ok := r.in.Mocks[n.ID]; ok {
		values := map[string]json.RawMessage{}
		_ = json.Unmarshal(mock.Body, &values)
		return resultOK, values
	}
	if r.s.Decisions == nil {
		return "ai_unavailable", nil
	}
	settings := settingsOf[ExtractSettings](n)
	fields := make([]aidecision.Field, 0, len(settings.Fields))
	for _, f := range settings.Fields {
		fields = append(fields, aidecision.Field{
			Key: f.Key, Label: f.Label, Description: f.Description, Type: f.Type, Options: f.Options,
		})
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(r.s.Config.AITimeoutSeconds, 1))*time.Second)
	defer cancel()
	got, err := r.s.Decisions.Extract(ctx, aidecision.Extraction{
		Subject: aidecision.Subject{
			WorkspaceID: r.ws, Context: profileOf(&run{people: []models.Contact{r.person}}),
		},
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
runAction answers the step with its mock, never the action, and keeps
the outputs it saves.
*/
func (r *simRun) runAction(n Node) outcome {
	s := settingsOf[RunActionSettings](n)
	mock, mocked := r.in.Mocks[n.ID]
	if !mocked {
		mock, mocked = r.in.Mocks[s.ActionID]
	}
	status := 200
	if mocked && mock.Status != 0 {
		status = mock.Status
	}
	inputs := map[string]string{}
	for _, in := range s.Inputs {
		inputs[in.Name] = fillDetails(in.Value, &r.person, nil)
	}
	r.act(n, map[string]any{"action_id": s.ActionID, "inputs": inputs, "status": status, "mocked": mocked})
	if status < 200 || status >= 300 {
		path := PathNext
		if r.g.hasPath(n.ID, PathFailed) {
			path = PathFailed
		}
		return outcome{path: path, result: "action:failed", details: map[string]any{"status": status}}
	}
	written := map[string]any{}
	if mocked {
		written = r.outputs(s, mock)
	}

	return outcome{path: PathNext, result: "action:ok", details: map[string]any{
		"status": status, "mocked": mocked, "values": written,
	}}
}

/*
outputs keeps the outputs a run action step saves, read from its mock.
*/
func (r *simRun) outputs(s RunActionSettings, mock actionkit.Mock) map[string]any {
	written := map[string]any{}
	var result map[string]json.RawMessage
	if json.Unmarshal(mock.Body, &result) != nil {
		return written
	}
	for _, sv := range s.Save {
		if raw, ok := result[sv.Output]; ok {
			var value any
			_ = json.Unmarshal(raw, &value)
			r.setAttribute(sv.Attribute, value)
			written[sv.Attribute] = value
		}
	}

	return written
}
