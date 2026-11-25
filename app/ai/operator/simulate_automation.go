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
| nothing is saved, sent or run for real.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	res, err := e.Services.WorkflowSims.Try(ctx, s.m, id, workflowsims.TryInput{
		Version: in.Version, Message: in.Message, Scenario: in.Situation, Persona: in.Customer,
	})
	if err != nil {
		return result{}, err
	}
	return result{text: workflowTryText(res), step: Step{Tool: "simulate_workflow", Summary: "Tried the workflow"}}, nil
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
	switch {
	case in.Contact != "":
		contact, err := e.publicID(in.Contact, models.PrefixContact)
		if err != nil {
			return result{}, err
		}
		sim.ContactID = &contact
	case in.Person != nil:
		sim.Person = in.Person
	default:
		sim.Person = &series.SimPerson{Name: "Test person"}
	}
	trace, err := e.Services.Series.Simulate(ctx, sim)
	if err != nil {
		return result{}, err
	}
	return result{text: seriesTryText(trace), step: Step{Tool: "simulate_series", Summary: "Tried the series"}}, nil
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
