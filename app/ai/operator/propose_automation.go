/*
|--------------------------------------------------------------------------
| Proposing Workflows And Series
|--------------------------------------------------------------------------
|
| Operator suggests a change to a workflow's or a series' working copy,
| or a whole new draft, as a changeset the teammate reviews in the
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
or the name and trigger or subject of a new one, and either a whole
spec or edits to the working copy.
*/
type automationInput struct {
	ID          string
	Name        string
	Description string
	Trigger     string
	Subject     string
	Spec        json.RawMessage
	Workflow    []workflowEdit
	Series      []seriesEdit
}

/*
newWorkflowDraft is the draft a new workflow starts as, kept on its
operation until a teammate accepts it.
*/
type newWorkflowDraft struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Trigger     string          `json:"trigger"`
	Spec        json.RawMessage `json:"spec"`
}

/*
newSeriesDraft is the draft a new series starts as.
*/
type newSeriesDraft struct {
	Name    string          `json:"name"`
	Subject string          `json:"subject"`
	Spec    json.RawMessage `json:"spec"`
}

/*
Tool names of the automation proposals.
*/
const (
	toolWorkflowEdit = "propose_workflow_edit"
	toolWorkflowNew  = "propose_new_workflow"
	toolSeriesEdit   = "propose_series_edit"
	toolSeriesNew    = "propose_new_series"
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
workflowNewSchema is the schema of propose_new_workflow.
*/
const workflowNewSchema = `{"type":"object","required":["suggestion_title","reason","name","trigger","spec"],
"properties":{
` + suggestionArgs + `,
"name":{"type":"string","maxLength":80,"description":"The workflow's name, unlike any other workflow's."},
"description":{"type":"string","maxLength":500,"description":"What it is for, in one line."},
"trigger":{"type":"string","description":"The trigger key it starts on, such as conversation.first_message. It ` +
	`never changes later."},
"spec":{"type":"object","description":"The whole definition: {trigger, trigger_settings, channels, audience, ` +
	`schedule, reentry, exclusive, steps}. Leave the ids of steps and paths out."}
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
seriesNewSchema is the schema of propose_new_series.
*/
const seriesNewSchema = `{"type":"object","required":["suggestion_title","reason","name","subject","spec"],
"properties":{
` + suggestionArgs + `,
"name":{"type":"string","maxLength":120,"description":"The series' name, unlike any other series'."},
"subject":{"type":"string","enum":["contact","company"],"description":"Who travels it. It never changes later."},
"spec":{"type":"object","description":"The whole definition: {entry, exit, goal, quiet_hours, nodes, edges}. ` +
	`nodes holds the step with the id entry and kind entry, then each step {id, kind, settings}; edges joins them, ` +
	`{from, path, to}."}
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
			}, writes: []Kind{KindWorkflowSet}, handle: e.proposeWorkflowEdit},
			tool{spec: contracts.ToolSpec{
				Name: toolWorkflowNew,
				Description: "Suggest a new draft workflow from what the teammate described. It is checked as " +
					"publishing would check it; accepting it creates the draft, which runs only once published.",
				Schema: schema(workflowNewSchema),
			}, writes: []Kind{KindWorkflowNew}, handle: e.proposeNewWorkflow})
	}
	if e.Services.Series != nil {
		out = append(out,
			tool{spec: contracts.ToolSpec{
				Name: toolSeriesEdit,
				Description: "Suggest a change to one series' working copy: add, change or remove steps, join " +
					"or cut paths, or change who enters, what makes them leave and the goal. It is checked as " +
					"publishing would check it, and shows on the teammate's builder for them to accept.",
				Schema: schema(seriesEditSchema),
			}, writes: []Kind{KindSeriesSet}, handle: e.proposeSeriesEdit},
			tool{spec: contracts.ToolSpec{
				Name: toolSeriesNew,
				Description: "Suggest a new draft series from what the teammate described. It is checked as " +
					"publishing would check it; accepting it creates the draft, which sends nothing until published.",
				Schema: schema(seriesNewSchema),
			}, writes: []Kind{KindSeriesNew}, handle: e.proposeNewSeries})
	}
	return out
}

/*
automationArgs are the arguments every automation proposal reads.
*/
type automationArgs struct {
	Title       string          `json:"suggestion_title"`
	Reason      string          `json:"reason"`
	DedupKey    string          `json:"dedup_key"`
	WorkflowID  string          `json:"workflow_id"`
	SeriesID    string          `json:"series_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Trigger     string          `json:"trigger"`
	Subject     string          `json:"subject"`
	Spec        json.RawMessage `json:"spec"`
	Edits       json.RawMessage `json:"edits"`
}

/*
proposeAutomation reads a proposal and stores it as one operation.
*/
func (e *Engine) proposeAutomation(ctx context.Context, s *session, args json.RawMessage, kind Kind) (result, error) {
	var in automationArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	a := &automationInput{
		ID: either(in.WorkflowID, in.SeriesID), Name: in.Name, Description: in.Description, Trigger: in.Trigger,
		Subject: in.Subject, Spec: objectArg(in.Spec),
	}
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

func (e *Engine) proposeNewWorkflow(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	return e.proposeAutomation(ctx, s, args, KindWorkflowNew)
}

func (e *Engine) proposeSeriesEdit(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	return e.proposeAutomation(ctx, s, args, KindSeriesSet)
}

func (e *Engine) proposeNewSeries(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	return e.proposeAutomation(ctx, s, args, KindSeriesNew)
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
