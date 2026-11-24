/*
|--------------------------------------------------------------------------
| Reading A Workflow Simulation
|--------------------------------------------------------------------------
|
| How a run reads to its checks: the chat as the judge reads it, with
| taps and silences written out, and what was observed, as the agent's
| checks read it.
|
*/

package workflowsims

import (
	"cmp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/support/procedure"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
	engine "github.com/dimmihq/dimmi/app/workflows"
)

/*
transcript is the chat as the judge reads it.
*/
func (c *chat) transcript() string {
	var b strings.Builder
	for _, l := range c.lines {
		switch {
		case l.Silent:
			b.WriteString("[The customer did not answer]\n")
		case l.From == "customer" && l.Tapped:
			b.WriteString("[The customer tapped the button: " + l.Text + "]\n")
		case l.From == "customer":
			b.WriteString("CUSTOMER: " + l.Text + "\n")
		case l.From == "workflow":
			b.WriteString("WORKFLOW: " + l.Text + "\n")
			if len(l.Choices) > 0 {
				b.WriteString("[Buttons: " + strings.Join(l.Choices, " | ") + "]\n")
			}
		default:
			b.WriteString(strings.ToUpper(cmp.Or(l.Name, "AGENT")) + ": " + l.Text + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

/*
observed is what the run did, as the agent's checks read it: the
workflow's actions as calls named by step id and kind, dimmi's own
calls, every reply and the handover.
*/
func (c *chat) observed(trace *engine.DryTrace) procedure.Observed {
	o := procedure.Observed{Branches: map[string]string{}, Calls: actionCalls(trace), Replies: []string{}}
	o.HandedOff = handedOverIn(trace)
	o.Calls = append(o.Calls, c.calls...)
	for _, l := range c.lines {
		if l.From != "customer" && l.Text != "" {
			o.Replies = append(o.Replies, l.Text)
		}
	}
	if c.handedOff {
		o.HandedOff, o.Team, o.Priority = true, c.team, c.priority
	}
	return o
}

/*
actionCalls are the workflow's actions as calls, named by step id and
kind.
*/
func actionCalls(trace *engine.DryTrace) []procedure.ObservedCall {
	out := []procedure.ObservedCall{}
	if trace == nil {
		return out
	}
	for _, a := range trace.Actions {
		out = append(out, procedure.ObservedCall{ActionID: a.StepID, Tool: a.Kind, Args: a.Detail, Succeeded: true})
	}
	return out
}

/*
handedOverIn reports whether the workflow ended in a handover, or an
agent step took its handed-over path.
*/
func handedOverIn(trace *engine.DryTrace) bool {
	if trace == nil {
		return false
	}
	return trace.End == engine.DryHandedOver || slices.ContainsFunc(trace.Steps, func(s engine.DryStep) bool {
		return s.Kind == workflowspec.KindDimmi && s.Outcome == workflowspec.OutcomeHandedOver
	})
}
