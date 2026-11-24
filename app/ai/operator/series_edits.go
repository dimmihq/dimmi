/*
|--------------------------------------------------------------------------
| Series Edits
|--------------------------------------------------------------------------
|
| A series is a graph: steps joined by paths. Operator changes it the way
| the builder does: it adds a step after another by one of its paths,
| which puts the new step between them; changes a step's settings;
| removes a step, joining what came before it to what came after; joins
| or cuts one path; and sets who enters, what makes them leave and the
| goal. Steps are named by the ids the series shows.
|
*/

package operator

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/series"
)

/*
The series edit actions besides the ones workflows share.
*/
const (
	editConnect    = "connect"
	editDisconnect = "disconnect"
	editSetEntry   = "set_entry"
	editSetExit    = "set_exit"
	editSetGoal    = "set_goal"
	editSetQuiet   = "set_quiet_hours"
)

/*
seriesEdit is one edit as the model sends it. Step is a new step;
StepID names the step it changes; FromStepID and Path say which path
leads to an added step, or which path to join or cut, and ToStepID
where a joined path leads.
*/
type seriesEdit struct {
	Action     string          `json:"action"`
	Step       *series.Node    `json:"step,omitempty"`
	StepID     string          `json:"step_id,omitempty"`
	FromStepID string          `json:"from_step_id,omitempty"`
	Path       string          `json:"path,omitempty"`
	ToStepID   string          `json:"to_step_id,omitempty"`
	Settings   json.RawMessage `json:"settings,omitempty"`
	Entry      *series.Entry   `json:"entry,omitempty"`
	Exit       *series.Exit    `json:"exit,omitempty"`
	Goal       json.RawMessage `json:"goal,omitempty"`
	Quiet      *bool           `json:"quiet_hours,omitempty"`
}

/*
applySeriesEdits applies edits in order to a copy of the spec.
*/
func applySeriesEdits(spec series.Spec, edits []seriesEdit) (series.Spec, error) {
	out := cloneSeries(spec)
	for i, e := range edits {
		var err error
		out, err = applySeriesEdit(out, e)
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
applySeriesEdit applies one edit.
*/
func applySeriesEdit(spec series.Spec, e seriesEdit) (series.Spec, error) {
	switch e.Action {
	case editAddStep:
		return addSeriesStep(spec, e)
	case editChangeStep:
		i := nodeIndex(spec, e.StepID)
		settings := objectArg(e.Settings)
		if i < 0 {
			return spec, unknownNode(e.StepID)
		}
		if len(settings) == 0 {
			return spec, fmt.Errorf("change_step needs the step's whole new settings")
		}
		spec.Nodes[i].Settings = settings
		return spec, nil
	case editRemoveStep:
		return removeSeriesStep(spec, e.StepID)
	case editConnect:
		if nodeIndex(spec, e.FromStepID) < 0 {
			return spec, unknownNode(e.FromStepID)
		}
		if nodeIndex(spec, e.ToStepID) < 0 {
			return spec, unknownNode(e.ToStepID)
		}
		spec.Edges = append(cutEdge(spec.Edges, e.FromStepID, pathOf(e.Path)),
			series.Edge{From: e.FromStepID, Path: pathOf(e.Path), To: e.ToStepID})
		return spec, nil
	case editDisconnect:
		spec.Edges = cutEdge(spec.Edges, e.FromStepID, pathOf(e.Path))
		return spec, nil
	case editSetEntry, editSetExit, editSetGoal, editSetQuiet:
		return setSeriesHead(spec, e)
	default:
		return spec, fmt.Errorf("unknown action %q; use add_step, change_step, remove_step, connect, "+
			"disconnect, set_entry, set_exit, set_goal or set_quiet_hours", e.Action)
	}
}

/*
addSeriesStep adds a step, and when it follows another, puts it on that
path: what the path led to before now follows the new step by next.
*/
func addSeriesStep(spec series.Spec, e seriesEdit) (series.Spec, error) {
	if e.Step == nil || !series.IsStepKind(e.Step.Kind) || e.Step.Kind == series.KindEntry {
		return spec, fmt.Errorf("add_step needs a step with a known kind other than entry")
	}
	n := *e.Step
	n.Settings = objectArg(n.Settings)
	if n.ID == "" {
		n.ID = freshNodeID(spec, n.Kind)
	}
	if nodeIndex(spec, n.ID) >= 0 {
		return spec, fmt.Errorf("a step %s is already in the series; leave the id out for a new step", n.ID)
	}
	n.Position = nil
	spec.Nodes = append(spec.Nodes, n)
	if e.FromStepID == "" {
		return spec, nil
	}
	if nodeIndex(spec, e.FromStepID) < 0 {
		return spec, unknownNode(e.FromStepID)
	}
	path := pathOf(e.Path)
	if from := spec.Nodes[nodeIndex(spec, e.FromStepID)]; !slices.Contains(series.StepPaths(from), path) {
		return spec, fmt.Errorf("step %s has no path %q; its paths are %s", from.ID, path,
			strings.Join(series.StepPaths(from), ", "))
	}
	was, led := edgeTo(spec.Edges, e.FromStepID, path)
	spec.Edges = append(cutEdge(spec.Edges, e.FromStepID, path), series.Edge{From: e.FromStepID, Path: path, To: n.ID})
	if led && slices.Contains(series.StepPaths(n), series.PathNext) {
		spec.Edges = append(spec.Edges, series.Edge{From: n.ID, Path: series.PathNext, To: was})
	}
	return spec, nil
}

/*
removeSeriesStep removes a step and every path to and from it; when one
path led in and it went on by next, the two are joined.
*/
func removeSeriesStep(spec series.Spec, id string) (series.Spec, error) {
	i := nodeIndex(spec, id)
	if i < 0 {
		return spec, unknownNode(id)
	}
	if id == series.KindEntry {
		return spec, fmt.Errorf("the entry step stays; change who enters with set_entry")
	}
	var in []series.Edge
	for _, edge := range spec.Edges {
		if edge.To == id {
			in = append(in, edge)
		}
	}
	next, goesOn := edgeTo(spec.Edges, id, series.PathNext)
	spec.Nodes = slices.Delete(spec.Nodes, i, i+1)
	spec.Edges = slices.DeleteFunc(spec.Edges, func(edge series.Edge) bool { return edge.From == id || edge.To == id })
	if len(in) == 1 && goesOn {
		spec.Edges = append(spec.Edges, series.Edge{From: in[0].From, Path: in[0].Path, To: next})
	}
	return spec, nil
}

/*
setSeriesHead sets who enters, what makes them leave, the goal or quiet
hours.
*/
func setSeriesHead(spec series.Spec, e seriesEdit) (series.Spec, error) {
	switch e.Action {
	case editSetEntry:
		if e.Entry == nil {
			return spec, fmt.Errorf("set_entry needs the whole entry")
		}
		spec.Entry = *e.Entry
	case editSetExit:
		if e.Exit == nil {
			return spec, fmt.Errorf("set_exit needs the whole exit")
		}
		spec.Exit = *e.Exit
	case editSetGoal:
		raw := objectArg(e.Goal)
		if len(raw) == 0 || string(raw) == "null" {
			spec.Goal = nil
			return spec, nil
		}
		var g series.Goal
		if err := json.Unmarshal(raw, &g); err != nil {
			return spec, fmt.Errorf("the goal does not fit its shape: %s", err.Error())
		}
		spec.Goal = &g
	default:
		if e.Quiet == nil {
			return spec, fmt.Errorf("set_quiet_hours needs quiet_hours true or false")
		}
		spec.Quiet = *e.Quiet
	}
	return spec, nil
}

/*
nodeIndex is where the step with the id sits, -1 when it is not there.
*/
func nodeIndex(spec series.Spec, id string) int {
	return slices.IndexFunc(spec.Nodes, func(n series.Node) bool { return n.ID == id })
}

/*
edgeTo is where one path of a step leads.
*/
func edgeTo(edges []series.Edge, from, path string) (string, bool) {
	for _, e := range edges {
		if e.From == from && pathOf(e.Path) == path {
			return e.To, true
		}
	}
	return "", false
}

/*
cutEdge removes one path of a step.
*/
func cutEdge(edges []series.Edge, from, path string) []series.Edge {
	return slices.DeleteFunc(edges, func(e series.Edge) bool { return e.From == from && pathOf(e.Path) == path })
}

/*
pathOf is a path's name, next when it is left out.
*/
func pathOf(path string) string {
	if strings.TrimSpace(path) == "" {
		return series.PathNext
	}
	return strings.TrimSpace(path)
}

/*
freshNodeID is an id for a new step of the kind no step has yet.
*/
func freshNodeID(spec series.Spec, kind string) string {
	base := strings.ReplaceAll(kind, "_", "-")
	for n := 1; ; n++ {
		id := fmt.Sprintf("%s-%d", base, n)
		if nodeIndex(spec, id) < 0 {
			return id
		}
	}
}

func unknownNode(id string) error {
	return fmt.Errorf("the series has no step %q; use a step id from its outline", id)
}

/*
cloneSeries copies a spec deeply.
*/
func cloneSeries(spec series.Spec) series.Spec {
	raw, err := json.Marshal(spec)
	if err != nil {
		return spec
	}
	var out series.Spec
	if json.Unmarshal(raw, &out) != nil {
		return spec
	}
	return series.Normalize(out)
}

/*
seriesOutline writes a series as the model edits it: who enters, what
makes them leave, the goal, and each step with its settings and where
each of its paths leads.
*/
func seriesOutline(spec series.Spec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Entry: %s\n", mustJSON(spec.Entry))
	fmt.Fprintf(&b, "Exit: %s\n", mustJSON(spec.Exit))
	if spec.Goal != nil {
		fmt.Fprintf(&b, "Goal: %s\n", mustJSON(spec.Goal))
	} else {
		b.WriteString("Goal: none\n")
	}
	fmt.Fprintf(&b, "Quiet hours: %t\nSteps:\n", spec.Quiet)
	for _, n := range spec.Nodes {
		fmt.Fprintf(&b, "- step %s · %s", n.ID, n.Kind)
		if len(n.Settings) > 0 {
			fmt.Fprintf(&b, " · settings %s", truncate(compactJSON(n.Settings), 2000))
		}
		b.WriteString("\n")
		for _, path := range series.StepPaths(n) {
			if to, ok := edgeTo(spec.Edges, n.ID, path); ok {
				fmt.Fprintf(&b, "    path %s → %s\n", path, to)
			} else {
				fmt.Fprintf(&b, "    path %s → (nothing: the journey ends)\n", path)
			}
		}
	}
	return b.String()
}

/*
mustJSON encodes a value for the model to read.
*/
func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(raw)
}
