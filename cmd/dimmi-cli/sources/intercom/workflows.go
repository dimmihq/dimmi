package intercom

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/engine"
	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
)

/*
workflowURL finds the ruleset id at the end of a workflow's address in
Intercom, .../automation/workflows/<id>.
*/
var workflowURL = regexp.MustCompile(`workflows/(\d+)`)

/*
WorkflowID reads a workflow's ruleset id from a pasted address or a bare
number.
*/
func WorkflowID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if m := workflowURL.FindStringSubmatch(s); m != nil {
		return m[1], true
	}

	if s != "" && strings.Trim(s, "0123456789") == "" {
		return s, true
	}

	return "", false
}

/*
stepTypes are the Intercom step and action types dimmi's workflows can
run, and what they become.
*/
var stepTypes = map[string]string{
	"send_message": "send_message", "reply": "send_message", "chat_message": "send_message", "message": "send_message",
	"add_tag": "add_tag", "tag_conversation": "add_tag", "remove_tag": "remove_tag",
	"assign": "assign", "assign_conversation": "assign", "assign_to_team": "assign", "assign_to_teammate": "assign",
	"close": "close", "close_conversation": "close", "snooze": "snooze", "snooze_conversation": "snooze",
	"set_attribute": "set_attribute", "set_conversation_data_attribute": "set_attribute",
	"note": "add_note", "add_note": "add_note", "wait": "wait", "branches": "branch", "condition": "branch",
	"collect_customer_data": "collect", "collect_data": "collect", "reply_buttons": "choice", "buttons": "choice",
	"priority": "set_priority", "mark_priority": "set_priority", "convert_to_ticket": "create_ticket",
}

/*
stepTypesIn walks a workflow export and collects every step or action
type it names, keeping only the type names, never their content.
*/
func stepTypesIn(raw json.RawMessage) []string {
	seen := map[string]bool{}

	var walk func(v any, key string)
	walk = func(v any, key string) {
		switch x := v.(type) {
		case map[string]any:
			for _, k := range []string{"step_type", "action_type", "type"} {
				if t, ok := x[k].(string); ok && t != "" && (k != "type" || key == "steps" || key == "actions") {
					seen[t] = true
				}
			}

			for k, child := range x {
				walk(child, k)
			}
		case []any:
			for _, child := range x {
				walk(child, key)
			}
		}
	}

	var v any
	if json.Unmarshal(raw, &v) == nil {
		walk(v, "")
	}

	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}

	sort.Strings(out)

	return out
}

type workflowExport struct {
	Workflow struct {
		ID          ID       `json:"id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		TriggerType string   `json:"trigger_type"`
		State       string   `json:"state"`
		Channels    []string `json:"target_channels"`
	} `json:"workflow"`
}

/*
translateWorkflow turns an exported workflow into a disabled dimmi
workflow, listing the steps that need rebuilding by hand.
*/
func translateWorkflow(rulesetID string, raw json.RawMessage) (format.Record, error) {
	var ex workflowExport
	if err := json.Unmarshal(raw, &ex); err != nil {
		return format.Record{}, err
	}

	wf := &format.Workflow{
		Base: format.Base{SourceID: rulesetID}, Name: firstOf(text(ex.Workflow.Title), "Workflow "+rulesetID),
		Description: text(ex.Workflow.Description), Trigger: ex.Workflow.TriggerType, Channels: ex.Workflow.Channels,
		Enabled: false, Definition: raw, Steps: []format.WorkflowStep{},
	}

	for _, t := range stepTypesIn(raw) {
		dimmi, ok := stepTypes[t]
		wf.Steps = append(wf.Steps, format.WorkflowStep{SourceType: t, Type: dimmi, Supported: ok})

		if !ok {
			wf.Unsupported = append(wf.Unsupported, t)
		}
	}

	return format.NewRecord(format.KindWorkflow, rulesetID, wf), nil
}

func (s *Source) readWorkflows(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
	var recs []format.Record

	for _, id := range s.Workflows {
		var raw json.RawMessage
		if err := s.Client.get(ctx, "/export/workflows/"+url.PathEscape(id), nil, &raw); err != nil {
			if isNotFound(err) {
				continue
			}

			return err
		}

		rec, err := translateWorkflow(id, raw)
		if err != nil {
			return err
		}

		recs = append(recs, rec)
	}

	return one(emit, recs)
}
