/*
|--------------------------------------------------------------------------
| Workflow And Series Reference
|--------------------------------------------------------------------------
|
| What the model needs to write a workflow or a series that checks
| clean, read from the same registries the checker reads: every trigger
| with the steps it may use, every step kind with its settings and the
| paths it has, and the fields a rule may read. Settings are written out
| from the JSON schemas of the settings types themselves, so the
| reference never drifts from what the checker accepts. The how-to that goes with it is a prompt
| file of its own.
|
*/

package operator

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/support/prompts"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The prompt files of workflows and series.
*/
const (
	taskWorkflows = "operator/workflows"
	taskSeries    = "operator/series"
)

/*
workflowSettingShapes writes each step kind's settings shape. A kind
missing here is described by its paths alone.
*/
func workflowSettingShapes() map[string]string {
	tags := shapeOf[workflowspec.TagSettings]()
	return map[string]string{
		workflowspec.KindMessage:         shapeOf[workflowspec.MessageSettings](),
		workflowspec.KindArticle:         shapeOf[workflowspec.ArticleSettings](),
		workflowspec.KindButtons:         shapeOf[workflowspec.ButtonsSettings](),
		workflowspec.KindCollect:         shapeOf[workflowspec.CollectSettings](),
		workflowspec.KindReplyTime:       shapeOf[workflowspec.ReplyTimeSettings](),
		workflowspec.KindDimmi:           shapeOf[workflowspec.DimmiSettings](),
		workflowspec.KindClassify:        shapeOf[workflowspec.ClassifySettings](),
		workflowspec.KindWait:            shapeOf[workflowspec.WaitSettings](),
		workflowspec.KindRunWorkflow:     shapeOf[workflowspec.RunWorkflowSettings](),
		workflowspec.KindAssign:          shapeOf[workflowspec.AssignSettings](),
		workflowspec.KindUnassign:        shapeOf[workflowspec.UnassignSettings](),
		workflowspec.KindTag:             tags,
		workflowspec.KindUntag:           tags,
		workflowspec.KindPriority:        shapeOf[workflowspec.PrioritySettings](),
		workflowspec.KindNote:            shapeOf[workflowspec.NoteSettings](),
		workflowspec.KindUpdateContact:   shapeOf[workflowspec.UpdateContactSettings](),
		workflowspec.KindSnooze:          shapeOf[workflowspec.SnoozeSettings](),
		workflowspec.KindClose:           shapeOf[workflowspec.CloseSettings](),
		workflowspec.KindApplySLA:        shapeOf[workflowspec.ApplySLASettings](),
		workflowspec.KindHTTPRequest:     shapeOf[workflowspec.HTTPRequestSettings](),
		workflowspec.KindCreateTicket:    shapeOf[workflowspec.CreateTicketSettings](),
		workflowspec.KindSetTicketState:  shapeOf[workflowspec.SetTicketStateSettings](),
		workflowspec.KindSetTicketFields: shapeOf[workflowspec.SetTicketFieldsSettings](),
		workflowspec.KindEscalate:        shapeOf[workflowspec.EscalateSettings](),
		workflowspec.KindAddToTracker:    shapeOf[workflowspec.AddToTrackerSettings](),
		workflowspec.KindExtract:         shapeOf[workflowspec.ExtractSettings](),
		workflowspec.KindRunAction:       shapeOf[workflowspec.RunActionSettings](),
		workflowspec.KindAssignAgent:     shapeOf[workflowspec.AssignAgentSettings](),
		workflowspec.KindTurnOffAI:       shapeOf[workflowspec.NoSettings](),
		workflowspec.KindTurnOnAI:        shapeOf[workflowspec.NoSettings](),
	}
}

/*
seriesSettingShapes writes each series step kind's settings shape.
*/
func seriesSettingShapes() map[string]string {
	message, action := shapeOf[series.MessageSettings](), shapeOf[series.ActionSettings]()
	return map[string]string{
		series.KindEmail: message, series.KindChat: message, series.KindPost: message, series.KindSlack: message,
		series.KindWait:         shapeOf[series.WaitSettings](),
		series.KindWaitFor:      shapeOf[series.WaitForSettings](),
		series.KindCondition:    shapeOf[series.ConditionSettings](),
		series.KindSplit:        shapeOf[series.SplitSettings](),
		series.KindAIPath:       shapeOf[series.AIPathSettings](),
		series.KindTag:          action,
		series.KindUntag:        action,
		series.KindSetAttribute: action,
		series.KindTopic:        action,
		series.KindWebhook:      shapeOf[series.WebhookSettings](),
		series.KindExtract:      shapeOf[series.ExtractSettings](),
		series.KindRunAction:    shapeOf[series.RunActionSettings](),
	}
}

/*
shapeOf writes a settings type as a compact JSON shape, from the JSON
schema the API would describe it with: each key with its type, a
question mark marking each key that may be left out.
*/
func shapeOf[T any]() string {
	schemas, err := routing.Schemas[T]()
	if err != nil {
		return "{}"
	}
	return writeShape(schemas, schemas[rootSchema(schemas)], 0)
}

/*
rootSchema is the name of the one schema no other schema refers to.
*/
func rootSchema(schemas map[string]any) string {
	raw, _ := json.Marshal(schemas)
	for name := range schemas {
		if !strings.Contains(string(raw), `"#/components/schemas/`+name+`"`) {
			return name
		}
	}
	return ""
}

/*
writeShape writes one schema as a shape.
*/
func writeShape(schemas map[string]any, node any, depth int) string {
	s, _ := node.(map[string]any)
	if ref, ok := s["$ref"].(string); ok && depth < 5 {
		return writeShape(schemas, schemas[strings.TrimPrefix(ref, "#/components/schemas/")], depth+1)
	}
	switch typeOf(s) {
	case "object":
		return objectShape(schemas, s, depth)
	case "array":
		return "[" + writeShape(schemas, s["items"], depth+1) + "]"
	case "string", "boolean", "number":
		return typeOf(s)
	case "integer":
		return "number"
	default:
		return "json"
	}
}

/*
objectShape writes an object schema's keys in order, the ones that may
be left out marked with a ?.
*/
func objectShape(schemas, s map[string]any, depth int) string {
	props, _ := s["properties"].(map[string]any)
	if len(props) == 0 {
		return "json"
	}
	required := map[string]bool{}
	list, _ := s["required"].([]any)
	for _, r := range list {
		if name, ok := r.(string); ok {
			required[name] = true
		}
	}
	parts := make([]string, 0, len(props))
	for _, name := range slices.Sorted(maps.Keys(props)) {
		mark := "?"
		if required[name] {
			mark = ""
		}
		parts = append(parts, name+mark+": "+writeShape(schemas, props[name], depth+1))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

/*
typeOf is a schema's type, the first that is not null when it lists
several.
*/
func typeOf(s map[string]any) string {
	switch t := s["type"].(type) {
	case string:
		return t
	case []any:
		for _, each := range t {
			if name, ok := each.(string); ok && name != "null" {
				return name
			}
		}
	}
	return ""
}

/*
triggerLine describes one trigger: its key and name, how often it may
run again, the channels it runs on and the steps that speak to the
customer it may use.
*/
func triggerLine(t workflowspec.Trigger) string {
	talks := "none (background only)"
	if len(t.Talks) > 0 {
		talks = strings.Join(t.Talks, ", ")
	}
	channels := "any"
	if len(t.Channels) > 0 {
		channels = strings.Join(t.Channels, ", ")
	}
	return fmt.Sprintf("- %s: %s. Group %s. Re-entry: %s (default %s). Channels: %s. Customer-facing steps it may "+
		"use: %s. dimmi step allowed: %t.", t.Key, t.Name, t.Group, strings.Join(t.Reentry, ", "), t.Default,
		channels, talks, t.Dimmi)
}

/*
kindLine describes one step kind under a trigger: its settings, its
paths, and whether the trigger may use it.
*/
func kindLine(k workflowspec.Kind, t *workflowspec.Trigger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- %s", k.Key)
	if t != nil && !t.AllowsStep(k.Key) {
		b.WriteString(" (not allowed under this trigger)")
	}
	if shape, ok := workflowSettingShapes()[k.Key]; ok {
		fmt.Fprintf(&b, ": settings %s", shape)
	}
	switch {
	case k.MaxPaths > 0:
		fmt.Fprintf(&b, ". Paths you add: %d to %d", k.MinPaths, k.MaxPaths)
	case len(k.Fixed) == 0:
		b.WriteString(". No paths: the next step in the list follows")
	}
	if len(k.Fixed) > 0 {
		fmt.Fprintf(&b, ". Fixed paths, by outcome: %s", strings.Join(k.Fixed, ", "))
	}
	if len(k.Required) > 0 {
		fmt.Fprintf(&b, " (always has %s)", strings.Join(k.Required, ", "))
	}
	b.WriteString(".")
	return b.String()
}

/*
workflowReference is everything the model needs to write workflow
steps: the how-to, the trigger, every step kind and the fields rules
read. With no trigger it lists every trigger instead.
*/
func workflowReference(trigger string) string {
	var b strings.Builder
	b.WriteString(prompts.Task(taskWorkflows, nil))
	t, found := workflowspec.FindTrigger(trigger)
	b.WriteString("\n\nTriggers:\n")
	for _, each := range workflowspec.Triggers() {
		if !found || each.Key == t.Key {
			b.WriteString(triggerLine(each) + "\n")
		}
	}
	var at *workflowspec.Trigger
	if found {
		at = &t
	}
	b.WriteString("\nStep kinds:\n")
	for _, k := range workflowspec.Kinds() {
		if k.Ready {
			b.WriteString(kindLine(k, at) + "\n")
		}
	}
	b.WriteString("\nFields a condition or audience may read (a key ending in . takes a name after it):\n")
	b.WriteString(fieldList())
	return b.String()
}

/*
fieldList lists the fields workflow rules read, with their types and
choices.
*/
func fieldList() string {
	var lines []string
	for _, f := range workflowspec.Catalog().Fields() {
		line := fmt.Sprintf("%s (%s", f.Key, f.Type)
		if len(f.Options) > 0 {
			line += ": " + strings.Join(f.Options, ", ")
		}
		lines = append(lines, line+")")
	}
	slices.Sort(lines)
	return strings.Join(lines, "; ") + "\n"
}

/*
seriesReference is everything the model needs to write series steps:
the how-to and every kind with its settings and paths.
*/
func seriesReference() string {
	var b strings.Builder
	b.WriteString(prompts.Task(taskSeries, nil))
	b.WriteString("\n\nStep kinds:\n")
	shapes := seriesSettingShapes()
	for _, kind := range []string{
		series.KindEmail, series.KindChat, series.KindPost, series.KindSlack, series.KindWait, series.KindWaitFor,
		series.KindCondition, series.KindSplit, series.KindAIPath, series.KindTag, series.KindUntag,
		series.KindSetAttribute, series.KindTopic, series.KindExtract, series.KindRunAction, series.KindExit,
	} {
		fmt.Fprintf(&b, "- %s", kind)
		if shape, ok := shapes[kind]; ok {
			fmt.Fprintf(&b, ": settings %s", shape)
		}
		paths := series.StepPaths(series.Node{Kind: kind})
		switch {
		case len(paths) == 0:
			b.WriteString(". No paths: the journey ends here")
		default:
			fmt.Fprintf(&b, ". Paths: %s", strings.Join(paths, ", "))
		}
		if kind == series.KindCondition || kind == series.KindSplit || kind == series.KindAIPath {
			b.WriteString(", plus one path per entry of its settings' paths or variants, by that entry's id")
		}
		b.WriteString(".\n")
	}
	fmt.Fprintf(&b, "\nEntry: %s\nExit: %s\nGoal: %s\n", shapeOf[series.Entry](), shapeOf[series.Exit](),
		shapeOf[series.Goal]())
	return b.String()
}
