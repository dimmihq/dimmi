/*
|--------------------------------------------------------------------------
| Trying Workflows And Series
|--------------------------------------------------------------------------
|
| Operator tries what it builds before it hands it over, the way a
| teammate would in the studio. simulate_workflow plays one chat: an AI
| customer writes the first message and plays the rest from the
| situation it is given. simulate_series walks one person, real or made
| up, through a series from a scenario in words; every wait is skipped.
| Both run against the draft or the live version and keep nothing:
| nothing is saved, sent or run for real. A try plays what Operator
| suggested in this chat, or on the suggestion the teammate has open,
| when there is one, so Operator checks its own change before the
| teammate accepts it. Each try is sent to the teammate as it happens,
| as operator.simulation events, and their page shows it live.
|
*/

package operator

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/actions/workflowsims"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
simulationTools are the tools that try a workflow or a series.
*/
func (e *Engine) simulationTools() []tool {
	var out []tool
	if e.Services.WorkflowSims != nil {
		out = append(out, tool{spec: contracts.ToolSpec{
			Name: "simulate_workflow",
			Description: "Try a workflow with one made-up chat before handing it over. An AI customer sends the " +
				"first message and plays the rest from the situation: it taps buttons, answers questions and " +
				"lets waits pass. Shows the chat, every step with the path it took, and what it would have done. " +
				"Nothing is saved, sent or run for real.",
			Schema: schema(`{"type":"object","required":["workflow_id","message","situation"],"properties":{` +
				`"workflow_id":{"type":"string","description":"Workflow id (wf_...)."},` +
				`"version":{"type":"string","enum":["draft","live"],"description":"Which copy. Draft by default."},` +
				`"message":{"type":"string","maxLength":4000,"description":"The customer's first message."},` +
				`"situation":{"type":"string","maxLength":1000,"description":"Who the customer is, what they ` +
				`want and how they answer what the workflow asks, such as: on the Pro plan, charged twice, ` +
				`picks the refund when offered."},` +
				`"customer":{"type":"object","description":"Who writes: name, email and custom attributes.",` +
				`"properties":{"name":{"type":"string"},"email":{"type":"string"},"attributes":{"type":"object"}}}}}`),
		}, handle: e.simulateWorkflow})
	}
	if e.Services.Series != nil {
		out = append(out, tool{spec: contracts.ToolSpec{
			Name: "simulate_series",
			Description: "Try a series by walking one person through it from a scenario in words, such as " +
				"opens every email but never clicks. Every wait is skipped. The AI reads the scenario for each " +
				"open, click, reply and event the series asks about. Shows each message they would get, what " +
				"they did, each path and how it ended. Nothing is saved or sent.",
			Schema: schema(`{"type":"object","required":["series_id","scenario"],"properties":{` +
				`"series_id":{"type":"string","description":"Series id (srs_...)."},` +
				`"version":{"type":"string","enum":["draft","live"],"description":"Which copy. Draft by default."},` +
				`"scenario":{"type":"string","maxLength":2000,"description":"How the person behaves, in words."},` +
				`"contact_id":{"type":"string","description":"A real contact to walk (cnt_...). Leave out for a ` +
				`made-up person."},` +
				`"person":{"type":"object","description":"A made-up person: name, email and custom attributes, ` +
				`set so they match who enters.","properties":{"name":{"type":"string"},"email":{"type":"string"},` +
				`"attributes":{"type":"object"}}}}}`),
		}, handle: e.simulateSeries})
	}
	return out
}

/*
EventSimulation is one moment of a try Operator runs, sent to the
teammate: started, a chat line, a step, and finished with the result.
*/
const EventSimulation = "operator.simulation"

/*
proposedDefinition is the newest definition Operator suggested for the
workflow or series, in this chat or on the suggestion the teammate has
open, and whether there is one.
*/
func proposedDefinition(s *session, kind Kind, id ids.UUID) (json.RawMessage, bool) {
	for _, row := range slices.Backward(s.proposed) {
		if def, ok := definitionIn(Operations(row.Operations), kind, id); ok {
			return def, true
		}
	}
	if s.viewed != nil {
		return definitionIn(Operations(s.viewed.Operations), kind, id)
	}
	return nil, false
}

/*
definitionIn is the newest definition among the operations for the
workflow or series.
*/
func definitionIn(ops []Operation, kind Kind, id ids.UUID) (json.RawMessage, bool) {
	for _, op := range slices.Backward(ops) {
		if op.Kind != kind {
			continue
		}
		about := op.Series
		if kind == KindWorkflowSet {
			about = op.Workflow
		}
		if about != nil && *about == id {
			return op.Definition, true
		}
	}
	return nil, false
}

/*
publishTry sends one moment of a try to the teammate.
*/
func (e *Engine) publishTry(ctx context.Context, s *session, data map[string]any) {
	data["thread_id"] = threadID(s)
	e.publish(ctx, s, fbroadcasting.Message{Event: EventSimulation, Ephemeral: true, Data: data})
}

/*
simulateWorkflow plays one chat through a workflow.
*/
func (e *Engine) simulateWorkflow(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Workflow  string          `json:"workflow_id"`
		Version   string          `json:"version"`
		Message   string          `json:"message"`
		Situation string          `json:"situation"`
		Customer  json.RawMessage `json:"customer"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	id, err := e.publicID(in.Workflow, models.PrefixWorkflow)
	if err != nil {
		return result{}, err
	}
	try := ids.Encode("try", ids.New())
	base := map[string]any{
		"kind": "workflow", "workflow_id": models.PublicID(models.PrefixWorkflow, id), "try_id": try,
		"message": in.Message, "situation": in.Situation,
	}
	spec, suggested := proposedDefinition(s, KindWorkflowSet, id)
	if in.Version == "live" {
		spec, suggested = nil, false
	}
	res, err := e.Services.WorkflowSims.Try(ctx, s.m, id, workflowsims.TryInput{
		Version: in.Version, Spec: spec, Message: in.Message, Scenario: in.Situation, Persona: in.Customer,
		Watch: func(ctx context.Context, moment string, details map[string]any) {
			data := maps.Clone(base)
			maps.Copy(data, details)
			data["moment"] = moment
			e.publishTry(ctx, s, data)
		},
	})
	if err != nil {
		return result{}, err
	}
	text := workflowTryText(res)
	if suggested {
		text = "This try played the change you suggested, which the teammate has not accepted yet.\n" + text
	}
	return result{text: text, step: Step{Tool: "simulate_workflow", Summary: "Tried the workflow"}}, nil
}

/*
workflowTryText writes a played chat: whether it started, the chat,
each step with its path, and what it would have done.
*/
func workflowTryText(res workflowsims.Result) string {
	var b strings.Builder
	if res.Error != "" {
		fmt.Fprintf(&b, "The try did not finish: %s\n", res.Error)
	}
	t := res.Trace
	if t == nil {
		return b.String()
	}
	if !t.Verdict.Started {
		fmt.Fprintf(&b, "The workflow would not start: %s %s\n", t.Verdict.Reason, t.Verdict.FailedRule)
		return b.String()
	}
	b.WriteString("Chat:\n")
	for _, l := range res.Transcript {
		text := l.Text
		if l.Silent {
			text = "(said nothing)"
		}
		fmt.Fprintf(&b, "- %s: %s\n", l.From, cutText(text, 400))
	}
	b.WriteString("\nSteps:\n")
	for _, st := range t.Steps {
		line := fmt.Sprintf("%s- %s (%s): %s", strings.Repeat("  ", st.Depth), st.StepID, st.Kind, st.Outcome)
		if st.PathLabel != "" || st.PathID != "" {
			line += ", path " + either(st.PathLabel, st.PathID)
		}
		if st.Skipped {
			line += ", skipped"
		}
		b.WriteString(line + "\n")
	}
	if len(t.Actions) > 0 {
		b.WriteString("\nWould have done:\n")
		for _, a := range t.Actions {
			fmt.Fprintf(&b, "- %s\n", a.Summary)
		}
	}
	fmt.Fprintf(&b, "\nEnded: %s\n", t.End)
	return b.String()
}

/*
simulateSeries walks one person through a series.
*/
func (e *Engine) simulateSeries(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Series   string            `json:"series_id"`
		Version  string            `json:"version"`
		Scenario string            `json:"scenario"`
		Contact  string            `json:"contact_id"`
		Person   *series.SimPerson `json:"person"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	id, err := e.publicID(in.Series, models.PrefixSeries)
	if err != nil {
		return result{}, err
	}
	sim := series.SimulateInput{Membership: s.m, ID: id, Version: in.Version, Scenario: strings.TrimSpace(in.Scenario)}
	if err := e.traveler(&sim, in.Contact, in.Person); err != nil {
		return result{}, err
	}
	if spec, suggested := proposedDefinition(s, KindSeriesSet, id); in.Version != "live" && suggested {
		sim.Spec = spec
	}
	base := map[string]any{
		"kind": "series", "series_id": models.PublicID(models.PrefixSeries, id), "try_id": ids.Encode("try", ids.New()),
		"scenario": sim.Scenario, "name": "A contact",
	}
	if sim.Person != nil {
		base["name"] = cmp.Or(sim.Person.Name, "Test person")
	}
	e.publishTry(ctx, s, withMoment(base, "started", nil))
	trace, err := e.Services.Series.Simulate(ctx, sim)
	if err != nil {
		e.publishTry(ctx, s, withMoment(base, "failed", map[string]any{"error": err.Error()}))
		return result{}, err
	}
	e.publishTry(ctx, s, withMoment(base, "finished", map[string]any{"result": trace}))
	text := seriesTryText(trace)
	if sim.Spec != nil {
		text = "This walk followed the change you suggested, which the teammate has not accepted yet.\n" + text
	}
	return result{text: text, step: Step{Tool: "simulate_series", Summary: "Tried the series"}}, nil
}

/*
traveler sets who walks a series: a real contact by id, the made-up
person given, or a test person.
*/
func (e *Engine) traveler(sim *series.SimulateInput, contact string, person *series.SimPerson) error {
	switch {
	case contact != "":
		id, err := e.publicID(contact, models.PrefixContact)
		if err != nil {
			return err
		}
		sim.ContactID = &id
	case person != nil:
		sim.Person = person
	default:
		sim.Person = &series.SimPerson{Name: "Test person"}
	}
	return nil
}

/*
withMoment is a copy of a try's details with the moment and what it
adds.
*/
func withMoment(base map[string]any, moment string, more map[string]any) map[string]any {
	data := maps.Clone(base)
	maps.Copy(data, more)
	data["moment"] = moment
	return data
}

/*
seriesTryText writes a walk in order: before each step what the person
did when asked, then the step, its path and the message it would send.
*/
func seriesTryText(t *series.SimTrace) string {
	var b strings.Builder
	if !t.Entered {
		fmt.Fprintf(&b, "They would not enter: %s.\n", t.Reason)
		return b.String()
	}
	messages := map[string]series.SimMessage{}
	for _, m := range t.Messages {
		messages[m.NodeID] = m
	}
	asked := func(at int) {
		for _, d := range t.Decisions {
			if d.Before == at {
				fmt.Fprintf(&b, "  (they %s: %s)\n", map[bool]string{true: "did", false: "did not"}[d.Did], d.What)
			}
		}
	}
	for i, st := range t.Steps {
		asked(i)
		line := fmt.Sprintf("- %s (%s): %s", st.NodeID, st.Kind, st.Result)
		if st.Path != "" && st.Path != "next" {
			line += ", path " + st.Path
		}
		if st.Skipped {
			line += ", skipped"
		}
		b.WriteString(line + "\n")
		if m, ok := messages[st.NodeID]; ok {
			fmt.Fprintf(&b, "  sends %q: %s\n", m.Subject, cutText(m.Body, 300))
		}
	}
	asked(len(t.Steps))
	fmt.Fprintf(&b, "\nEnded: %s. Goal reached: %t.\n", t.End, t.Goal)
	return b.String()
}

/*
cutText shortens a text to n runes.
*/
func cutText(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
