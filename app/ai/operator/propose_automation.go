/*
|--------------------------------------------------------------------------
| Proposing Workflows And Series
|--------------------------------------------------------------------------
|
| Operator suggests a change to a workflow's or a series' working copy
| as a changeset the teammate reviews in the
| builder. Edits are applied to the working copy (or to the suggestion
| the teammate has open on it), and the result is checked the way
| publishing checks it: an edit that adds a problem publishing would
| refuse is refused with every problem and the step it is on, so the
| model fixes it and sends it again. Problems the working copy already
| had are left to the teammate, and said. The model reads the workflow
| as it would read after the edit, with every new id.
|
*/

package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
automationInput is a proposal about a workflow or a series: which one,
and either a whole spec or edits to the working copy.
*/
type automationInput struct {
	ID       string
	Spec     json.RawMessage
	Workflow []workflowEdit
	Series   []seriesEdit
}

/*
Tool names of the automation proposals.
*/
const (
	toolWorkflowEdit = "propose_workflow_edit"
	toolSeriesEdit   = "propose_series_edit"
)

/*
suggestionArgs are the parts every proposal schema shares.
*/
const suggestionArgs = `"suggestion_title":{"type":"string","maxLength":200,"description":"What it does in a few ` +
	`everyday words, like 'Route billing questions to Billing'."},
"reason":{"type":"string","maxLength":2000,"description":"` + reasonHelp + `"},
"dedup_key":{"type":"string","maxLength":200,"description":"A stable key for what this suggests. A suggestion ` +
	`with the same key that still waits, or was declined in the last 30 days, is not made again."}`

/*
workflowEditSchema is the schema of propose_workflow_edit.
*/
const workflowEditSchema = `{"type":"object","required":["suggestion_title","reason","workflow_id"],"properties":{
` + suggestionArgs + `,
"workflow_id":{"type":"string","description":"Workflow id (wf_...)."},
"edits":{"type":"array","maxItems":50,"description":"The changes, applied in order to the working copy.",
"items":{"type":"object","required":["action"],"properties":{
"action":{"type":"string","enum":["add_step","change_step","remove_step","move_step","add_path","change_path",` +
	`"remove_path","set_trigger"]},
"step":{"type":"object","description":"For add_step: the new step, {kind, settings, paths}. Leave its id and its ` +
	`paths' ids out. A path is {label, condition, description, outcome, steps}."},
"step_id":{"type":"string","description":"The step to change, remove or move, or whose path to change (s_...)."},
"path_id":{"type":"string","description":"For change_path and remove_path: the path (p_...)."},
"after_step_id":{"type":"string","description":"For add_step and move_step: put it right after this step."},
"into_path_id":{"type":"string","description":"For add_step and move_step: put it into this path, last."},
"at_start":{"type":"boolean","description":"With into_path_id, or alone for the top list: put it first."},
"settings":{"type":"object","description":"For change_step: the step's whole new settings."},
"path":{"type":"object","description":"For add_path: the new path, {label, condition, description, outcome, ` +
	`steps}."},
"label":{"type":"string","description":"For change_path: the new label."},
"condition":{"type":"object","description":"For change_path on a branch: the new condition rule."},
"description":{"type":"string","description":"For change_path on classify: the category's new description."},
"trigger_settings":{"type":"object","description":"For set_trigger: the trigger's own settings."},
"channels":{"type":"array","items":{"type":"string"},"description":"For set_trigger: the channels it runs on; ` +
	`empty for all."},
"audience":{"description":"For set_trigger: who it runs for, {visitors, rules}, or null for everyone."},
"schedule":{"type":"object","description":"For set_trigger: {mode: any, open or closed, schedule_id}."},
"reentry":{"type":"object","description":"For set_trigger: {mode, cooldown}."},
"exclusive":{"type":"boolean","description":"For set_trigger: stop later workflows once this one starts."}
}}},
"spec":{"type":"object","description":"Instead of edits, the whole new working copy, only to rewrite most of it."}
}}`

/*
seriesEditSchema is the schema of propose_series_edit.
*/
const seriesEditSchema = `{"type":"object","required":["suggestion_title","reason","series_id"],"properties":{
` + suggestionArgs + `,
"series_id":{"type":"string","description":"Series id (srs_...)."},
"edits":{"type":"array","maxItems":50,"description":"The changes, applied in order to the working copy.",
"items":{"type":"object","required":["action"],"properties":{
"action":{"type":"string","enum":["add_step","change_step","remove_step","connect","disconnect","set_entry",` +
	`"set_exit","set_goal","set_quiet_hours"]},
"step":{"type":"object","description":"For add_step: the new step, {kind, settings}. Leave its id out."},
"step_id":{"type":"string","description":"For change_step and remove_step: the step."},
"from_step_id":{"type":"string","description":"For add_step: the step the new one follows. For connect and ` +
	`disconnect: the step the path leaves."},
"path":{"type":"string","description":"The path of from_step_id, such as next, matched or a condition path's ` +
	`id; next when left out."},
"to_step_id":{"type":"string","description":"For connect: where the path leads."},
"settings":{"type":"object","description":"For change_step: the step's whole new settings."},
"entry":{"type":"object","description":"For set_entry: the whole entry."},
"exit":{"type":"object","description":"For set_exit: the whole exit."},
"goal":{"description":"For set_goal: the whole goal, or null for none."},
"quiet_hours":{"type":"boolean","description":"For set_quiet_hours."}
}}},
"spec":{"type":"object","description":"Instead of edits, the whole new working copy, only to rewrite most of it."}
}}`

/*
automationTools are the tools that suggest workflow and series changes.
*/
func (e *Engine) automationTools() []tool {
	var out []tool
	if e.Services.Workflows != nil {
		out = append(out,
			tool{spec: contracts.ToolSpec{
				Name: toolWorkflowEdit,
				Description: "Suggest a change to one workflow's working copy: add, change, remove or move steps, " +
					"add, change or remove paths, or change the trigger's settings. It is checked as publishing " +
					"would check it, and shows on the teammate's builder for them to accept. Call it once per " +
					"workflow.",
				Schema: schema(workflowEditSchema),
			}, writes: []Kind{KindWorkflowSet}, handle: e.proposeWorkflowEdit})
	}
	if e.Services.Series != nil {
		out = append(out,
			tool{spec: contracts.ToolSpec{
				Name: toolSeriesEdit,
				Description: "Suggest a change to one series' working copy: add, change or remove steps, join " +
					"or cut paths, or change who enters, what makes them leave and the goal. It is checked as " +
					"publishing would check it, and shows on the teammate's builder for them to accept.",
				Schema: schema(seriesEditSchema),
			}, writes: []Kind{KindSeriesSet}, handle: e.proposeSeriesEdit})
	}
	return out
}

/*
automationArgs are the arguments every automation proposal reads.
*/
type automationArgs struct {
	Title      string          `json:"suggestion_title"`
	Reason     string          `json:"reason"`
	DedupKey   string          `json:"dedup_key"`
	WorkflowID string          `json:"workflow_id"`
	SeriesID   string          `json:"series_id"`
	Spec       json.RawMessage `json:"spec"`
	Edits      json.RawMessage `json:"edits"`
}

/*
proposeAutomation reads a proposal and stores it as one operation.
*/
func (e *Engine) proposeAutomation(ctx context.Context, s *session, args json.RawMessage, kind Kind) (result, error) {
	var in automationArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	a := &automationInput{ID: either(in.WorkflowID, in.SeriesID), Spec: objectArg(in.Spec)}
	if edits := listArg(in.Edits); len(edits) > 0 {
		var err error
		if kind == KindSeriesSet {
			err = json.Unmarshal(edits, &a.Series)
		} else {
			err = json.Unmarshal(edits, &a.Workflow)
		}
		if err != nil {
			return result{}, fmt.Errorf("%w: the edits do not fit their shape: %s", errBadArguments, err.Error())
		}
	}
	return e.propose(ctx, s, proposal{
		Title: in.Title, Reason: in.Reason, DedupKey: in.DedupKey, Operations: []opInput{{Kind: kind, auto: a}},
	})
}

func (e *Engine) proposeWorkflowEdit(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	return e.proposeAutomation(ctx, s, args, KindWorkflowSet)
}

func (e *Engine) proposeSeriesEdit(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	return e.proposeAutomation(ctx, s, args, KindSeriesSet)
}

/*
listArg reads an argument meant as a JSON array that a model sent
encoded as a string instead.
*/
func listArg(raw json.RawMessage) json.RawMessage {
	raw = objectArg(raw)
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil
	}
	return raw
}
