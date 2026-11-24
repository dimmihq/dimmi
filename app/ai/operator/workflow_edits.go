/*
|--------------------------------------------------------------------------
| Workflow Edits
|--------------------------------------------------------------------------
|
| Operator changes a workflow the way a teammate does in the builder: it
| adds a step after another or into a path, changes one step's settings,
| removes a step with what hangs under it, moves a step, adds, changes or
| removes a path, and changes the trigger's settings. Each edit names
| steps and paths by the ids the workflow shows, so a long workflow is
| never written out again to change one line. The edits are applied to
| the working copy in order, and the result is checked as a whole.
|
*/

package operator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The workflow edit actions.
*/
const (
	editAddStep    = "add_step"
	editChangeStep = "change_step"
	editRemoveStep = "remove_step"
	editMoveStep   = "move_step"
	editAddPath    = "add_path"
	editChangePath = "change_path"
	editRemovePath = "remove_path"
	editSetTrigger = "set_trigger"
)

/*
workflowEdit is one edit as the model sends it. Step and Path are the
new step or path; StepID and PathID name what it changes; AfterStepID,
IntoPathID and AtStart say where an added or moved step goes; the rest
are the trigger's settings.
*/
type workflowEdit struct {
	Action          string                 `json:"action"`
	Step            *workflowspec.Step     `json:"step,omitempty"`
	Path            *workflowspec.Path     `json:"path,omitempty"`
	StepID          string                 `json:"step_id,omitempty"`
	PathID          string                 `json:"path_id,omitempty"`
	AfterStepID     string                 `json:"after_step_id,omitempty"`
	IntoPathID      string                 `json:"into_path_id,omitempty"`
	AtStart         bool                   `json:"at_start,omitempty"`
	Settings        json.RawMessage        `json:"settings,omitempty"`
	Label           *string                `json:"label,omitempty"`
	Condition       json.RawMessage        `json:"condition,omitempty"`
	Description     *string                `json:"description,omitempty"`
	TriggerSettings json.RawMessage        `json:"trigger_settings,omitempty"`
	Channels        *[]string              `json:"channels,omitempty"`
	Audience        json.RawMessage        `json:"audience,omitempty"`
	Schedule        *workflowspec.Schedule `json:"schedule,omitempty"`
	Reentry         *workflowspec.Reentry  `json:"reentry,omitempty"`
	Exclusive       *bool                  `json:"exclusive,omitempty"`
}

/*
applyWorkflowEdits applies edits in order to a copy of the spec.
*/
func applyWorkflowEdits(spec workflowspec.Spec, edits []workflowEdit) (workflowspec.Spec, error) {
	out := cloneSpec(spec)
	for i, e := range edits {
		var err error
		out, err = applyWorkflowEdit(out, e)
		if err != nil {
			if len(edits) > 1 {
				return out, fmt.Errorf("%w: edit %d (%s): %s", errBadArguments, i+1, e.Action, err.Error())
			}
			return out, fmt.Errorf("%w: %s", errBadArguments, err.Error())
		}
	}
	return out, nil
}

/*
applyWorkflowEdit applies one edit.
*/
func applyWorkflowEdit(spec workflowspec.Spec, e workflowEdit) (workflowspec.Spec, error) {
	switch e.Action {
	case editAddStep:
		if e.Step == nil || strings.TrimSpace(e.Step.Kind) == "" {
			return spec, fmt.Errorf("add_step needs a step with its kind")
		}
		step := *e.Step
		step.Settings = objectArg(step.Settings)
		return placeStep(spec, step, e)
	case editChangeStep:
		return changeStep(spec, e)
	case editRemoveStep:
		next, _, ok := takeStep(spec.Steps, e.StepID)
		if !ok {
			return spec, unknownStep(e.StepID)
		}
		spec.Steps = next
		return spec, nil
	case editMoveStep:
		next, step, ok := takeStep(spec.Steps, e.StepID)
		if !ok {
			return spec, unknownStep(e.StepID)
		}
		spec.Steps = next
		return placeStep(spec, step, e)
	case editAddPath, editChangePath, editRemovePath:
		return editPath(spec, e)
	case editSetTrigger:
		return setTrigger(spec, e), nil
	default:
		return spec, fmt.Errorf("unknown action %q; use add_step, change_step, remove_step, move_step, "+
			"add_path, change_path, remove_path or set_trigger", e.Action)
	}
}

/*
placeStep puts a step after another, into a path, or into the top list,
at its end unless at_start says first.
*/
func placeStep(spec workflowspec.Spec, step workflowspec.Step, e workflowEdit) (workflowspec.Spec, error) {
	if step.ID != "" && hasStep(spec.Steps, step.ID) {
		return spec, fmt.Errorf("a step %s is already in the workflow; leave the id out for a new step", step.ID)
	}
	if e.AfterStepID != "" {
		var placed bool
		spec.Steps, placed = insertAfter(spec.Steps, e.AfterStepID, step)
		if !placed {
			return spec, unknownStep(e.AfterStepID)
		}
		return spec, nil
	}
	if e.IntoPathID != "" {
		var placed bool
		spec.Steps, placed = insertInto(spec.Steps, e.IntoPathID, step, placement{first: e.AtStart})
		if !placed {
			return spec, unknownPath(e.IntoPathID)
		}
		return spec, nil
	}
	spec.Steps = placement{first: e.AtStart}.add(spec.Steps, step)
	return spec, nil
}

/*
placement says where in a list a step goes: first, or last.
*/
type placement struct{ first bool }

/*
add puts a step into a list where the placement says.
*/
func (p placement) add(list []workflowspec.Step, step workflowspec.Step) []workflowspec.Step {
	if p.first {
		return append([]workflowspec.Step{step}, list...)
	}
	return append(list, step)
}

/*
insertAfter puts a step right after the one with the id, wherever it is.
*/
func insertAfter(list []workflowspec.Step, after string, step workflowspec.Step) ([]workflowspec.Step, bool) {
	for i := range list {
		if list[i].ID == after {
			return slices.Insert(list, i+1, step), true
		}
		for j := range list[i].Paths {
			if next, ok := insertAfter(list[i].Paths[j].Steps, after, step); ok {
				list[i].Paths[j].Steps = next
				return list, true
			}
		}
	}
	return list, false
}

/*
insertInto puts a step into the path with the id.
*/
func insertInto(list []workflowspec.Step, path string, step workflowspec.Step, at placement,
) ([]workflowspec.Step, bool) {
	for i := range list {
		for j := range list[i].Paths {
			p := &list[i].Paths[j]
			if p.ID == path {
				p.Steps = at.add(p.Steps, step)
				return list, true
			}
			if next, ok := insertInto(p.Steps, path, step, at); ok {
				p.Steps = next
				return list, true
			}
		}
	}
	return list, false
}

/*
takeStep removes the step with the id, with its paths, and returns it.
*/
func takeStep(list []workflowspec.Step, id string) ([]workflowspec.Step, workflowspec.Step, bool) {
	for i := range list {
		if list[i].ID == id {
			step := list[i]
			return slices.Delete(list, i, i+1), step, true
		}
		for j := range list[i].Paths {
			if next, step, ok := takeStep(list[i].Paths[j].Steps, id); ok {
				list[i].Paths[j].Steps = next
				return list, step, true
			}
		}
	}
	return list, workflowspec.Step{}, false
}

/*
findStep returns the step with the id, wherever it is.
*/
func findStep(list []workflowspec.Step, id string) *workflowspec.Step {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
		for j := range list[i].Paths {
			if found := findStep(list[i].Paths[j].Steps, id); found != nil {
				return found
			}
		}
	}
	return nil
}

/*
hasStep reports whether a step with the id is anywhere in the list.
*/
func hasStep(list []workflowspec.Step, id string) bool {
	return findStep(list, id) != nil
}

/*
changeStep replaces one step's settings.
*/
func changeStep(spec workflowspec.Spec, e workflowEdit) (workflowspec.Spec, error) {
	step := findStep(spec.Steps, e.StepID)
	if step == nil {
		return spec, unknownStep(e.StepID)
	}
	settings := objectArg(e.Settings)
	if len(settings) == 0 {
		return spec, fmt.Errorf("change_step needs the step's whole new settings")
	}
	step.Settings = settings
	return spec, nil
}

/*
editPath adds, changes or removes one path of a step.
*/
func editPath(spec workflowspec.Spec, e workflowEdit) (workflowspec.Spec, error) {
	step := findStep(spec.Steps, e.StepID)
	if step == nil {
		return spec, unknownStep(e.StepID)
	}
	if e.Action == editAddPath {
		if e.Path == nil {
			return spec, fmt.Errorf("add_path needs a path")
		}
		p := *e.Path
		p.Condition = objectArg(p.Condition)
		if p.Steps == nil {
			p.Steps = []workflowspec.Step{}
		}
		step.Paths = append(step.Paths, p)
		return spec, nil
	}
	i := slices.IndexFunc(step.Paths, func(p workflowspec.Path) bool { return p.ID == e.PathID })
	if i < 0 {
		return spec, fmt.Errorf("step %s has no path %q; its paths are %s", step.ID, e.PathID, pathList(*step))
	}
	if e.Action == editRemovePath {
		step.Paths = slices.Delete(step.Paths, i, i+1)
		return spec, nil
	}
	p := &step.Paths[i]
	if e.Label != nil {
		p.Label = *e.Label
	}
	if c := objectArg(e.Condition); len(c) > 0 {
		p.Condition = c
	}
	if e.Description != nil {
		p.Description = *e.Description
	}
	return spec, nil
}

/*
setTrigger changes what starts the workflow and when it may run, never
which trigger it is.
*/
func setTrigger(spec workflowspec.Spec, e workflowEdit) workflowspec.Spec {
	if raw := objectArg(e.TriggerSettings); len(raw) > 0 {
		spec.TriggerSettings = raw
	}
	if e.Channels != nil {
		spec.Channels = *e.Channels
	}
	if raw := objectArg(e.Audience); len(raw) > 0 {
		spec.Audience = raw
		if string(raw) == "null" {
			spec.Audience = nil
		}
	}
	if e.Schedule != nil {
		spec.Schedule = *e.Schedule
	}
	if e.Reentry != nil {
		spec.Reentry = *e.Reentry
	}
	if e.Exclusive != nil {
		spec.Exclusive = *e.Exclusive
	}
	return spec
}

/*
pathList names a step's paths for a refusal.
*/
func pathList(s workflowspec.Step) string {
	if len(s.Paths) == 0 {
		return "none"
	}
	names := make([]string, len(s.Paths))
	for i, p := range s.Paths {
		names[i] = p.ID + " (" + either(p.Label, p.Outcome) + ")"
	}
	return strings.Join(names, ", ")
}

func unknownStep(id string) error {
	return fmt.Errorf("the workflow has no step %q; use a step id from its outline (s_...)", id)
}

func unknownPath(id string) error {
	return fmt.Errorf("the workflow has no path %q; use a path id from its outline (p_...)", id)
}

/*
cloneSpec copies a spec deeply, so edits never touch what was read.
*/
func cloneSpec(spec workflowspec.Spec) workflowspec.Spec {
	raw, err := json.Marshal(spec)
	if err != nil {
		return spec
	}
	var out workflowspec.Spec
	if json.Unmarshal(raw, &out) != nil {
		return spec
	}
	if out.Steps == nil {
		out.Steps = []workflowspec.Step{}
	}
	return out
}

/*
workflowOutline writes a workflow's steps as an indented list: each
step's id, kind and settings, and each path's id, label or outcome, with
the steps it holds below it. It is what the model edits by.
*/
func workflowOutline(spec workflowspec.Spec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Trigger: %s\n", spec.Trigger)
	if len(spec.TriggerSettings) > 0 {
		fmt.Fprintf(&b, "Trigger settings: %s\n", compactJSON(spec.TriggerSettings))
	}
	if len(spec.Channels) > 0 {
		fmt.Fprintf(&b, "Channels: %s\n", strings.Join(spec.Channels, ", "))
	}
	if len(spec.Audience) > 0 {
		fmt.Fprintf(&b, "Audience: %s\n", compactJSON(spec.Audience))
	}
	fmt.Fprintf(&b, "Schedule: %s%s · Re-entry: %s%s · Exclusive: %t\n", either(spec.Schedule.Mode, "any"),
		prefixed(" ", spec.Schedule.ScheduleID), either(spec.Reentry.Mode, "the trigger's default"),
		prefixed(" ", spec.Reentry.Cooldown), spec.Exclusive)
	b.WriteString("Steps:\n")
	if len(spec.Steps) == 0 {
		b.WriteString("  (none yet)\n")
	}
	writeSteps(&b, spec.Steps, 1)
	return b.String()
}

/*
writeSteps writes a list of steps at a depth.
*/
func writeSteps(b *strings.Builder, steps []workflowspec.Step, depth int) {
	pad := strings.Repeat("  ", depth)
	for _, s := range steps {
		fmt.Fprintf(b, "%s- step %s · %s", pad, s.ID, s.Kind)
		if len(s.Settings) > 0 {
			fmt.Fprintf(b, " · settings %s", truncate(compactJSON(s.Settings), 1500))
		}
		b.WriteString("\n")
		for _, p := range s.Paths {
			writePath(b, p, depth)
		}
	}
}

/*
writePath writes one path and the steps it holds.
*/
func writePath(b *strings.Builder, p workflowspec.Path, depth int) {
	pad := strings.Repeat("  ", depth)
	fmt.Fprintf(b, "%s  path %s", pad, p.ID)
	for _, part := range [][2]string{
		{"outcome", p.Outcome},
		{"label", quoted(p.Label)},
		{"description", quoted(truncate(p.Description, 300))},
		{"condition", truncate(compactJSON(p.Condition), 800)},
	} {
		if part[1] != "" {
			fmt.Fprintf(b, " · %s %s", part[0], part[1])
		}
	}
	b.WriteString("\n")
	if len(p.Steps) == 0 {
		fmt.Fprintf(b, "%s    (no steps: the run ends here)\n", pad)
	}
	writeSteps(b, p.Steps, depth+2)
}

/*
quoted is a text in quotes, or nothing when it is empty.
*/
func quoted(text string) string {
	if text == "" {
		return ""
	}
	return fmt.Sprintf("%q", text)
}

/*
prefixed is a value with a prefix, or nothing when it is empty.
*/
func prefixed(prefix, value string) string {
	if value == "" {
		return ""
	}
	return prefix + value
}

/*
compactJSON is JSON without spaces, as the model reads it.
*/
func compactJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return string(raw)
	}
	return b.String()
}
