/*
|--------------------------------------------------------------------------
| Dry Simulations From a Scenario
|--------------------------------------------------------------------------
|
| A teammate can describe how the person behaves in words, such as
| "opens every email, clicks the pricing link in the second, never
| replies", instead of ticking what they do. Each time the series asks
| whether the person opened, clicked, replied or did an event, the
| decision model reads the scenario, the person and the messages they
| got so far, and answers yes or no. The person enters an event-started
| series as if they did its event, and never engages with a message they
| did not get. Answers are kept on the trace, at the step they were
| asked before, so the walk reads as what they did; the checks asked
| before every step, a reply that exits and the goal's event, are kept
| only when the person did it.
|
*/

package series

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrSimUndecided says the decision model could not answer for the
scenario.
*/
var ErrSimUndecided = fault.New(fault.Unavailable, "series_simulation_undecided",
	"The AI could not read the scenario just now. Try again in a moment.")

/*
The options the decision model picks from.
*/
const (
	choiceYes = "yes"
	choiceNo  = "no"
)

/*
check is what a wait-for step or a check's path asks: rules to match,
an event to do and a message to engage with, whichever are given, and
what the series calls it.
*/
type check struct {
	rules      json.RawMessage
	event      string
	engagement *Engagement
	label      string
}

/*
asking is how a question is put to the model: an aside it reads that
the trace does not show, such as what the series calls the check, and
whether the answer is kept on the trace only when it is yes, as for
the checks made before every step.
*/
type asking struct {
	aside string
	quiet bool
}

/*
test reports whether the person matches the rules, would do the event
and engaged as told, whichever are given.
*/
func (r *simRun) test(ctx context.Context, c check) (bool, error) {
	if len(c.rules) > 0 {
		ok, err := r.matches(ctx, nil, c.rules)
		if err != nil || !ok {
			return false, err
		}
	}
	if c.event != "" {
		did, err := r.did(ctx, c.event, asking{})
		if err != nil || !did {
			return false, err
		}
	}
	if c.engagement == nil {
		return len(c.rules) > 0 || c.event != "", nil
	}
	return r.engaged(ctx, c.engagement, asking{aside: calledAside(c.label)})
}

/*
did reports whether the person does the event: as the scenario says,
or as the teammate ticked.
*/
func (r *simRun) did(ctx context.Context, event string, a asking) (bool, error) {
	if r.in.Scenario == "" {
		return slices.Contains(r.in.Events, event), nil
	}
	what, aside := r.eventWords(ctx, event)
	a.aside += aside
	return r.does(ctx, what, a)
}

/*
engaged reports whether the person engages with a message: as the
scenario says, or as the teammate ticked. Nobody engages with a
message they did not get.
*/
func (r *simRun) engaged(ctx context.Context, e *Engagement, a asking) (bool, error) {
	if r.in.Scenario == "" {
		return slices.Contains(r.in.Engaged, e.Action), nil
	}
	if !r.got(e.Node) {
		return false, nil
	}
	return r.does(ctx, r.engagementWords(e), a)
}

/*
does asks the decision model whether the person, as the scenario
describes them, does what is asked at this point. Asked again with no
new message in between, the same question gets the same answer.
*/
func (r *simRun) does(ctx context.Context, what string, a asking) (bool, error) {
	key := fmt.Sprintf("%s#%d", what, len(r.out.Messages))
	if did, ok := r.decided[key]; ok {
		return did, nil
	}
	if r.s.Decisions == nil {
		return false, ErrSimUndecided
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(r.s.Config.AITimeoutSeconds, 1))*time.Second)
	defer cancel()
	chosen, err := r.s.Decisions.Choose(ctx, aidecision.Choice{
		Subject: aidecision.Subject{WorkspaceID: r.ws, Context: r.scenarioContext()},
		Instructions: "A teammate is testing an email series on a pretend person. The scenario says how this " +
			"person behaves. Going by the scenario and the messages they got so far, answer whether they " +
			"do this now: " + what + a.aside + ". When the scenario does not say, answer what such a " +
			"person would most likely do.",
		Options: []aidecision.Option{
			{ID: choiceYes, Label: "Yes", Description: "They do it."},
			{ID: choiceNo, Label: "No", Description: "They do not do it."},
		},
	})
	if err != nil || (chosen.OptionID != choiceYes && chosen.OptionID != choiceNo) {
		return false, ErrSimUndecided
	}
	did := chosen.OptionID == choiceYes
	r.decided[key] = did
	if did || !a.quiet {
		r.out.Decisions = append(r.out.Decisions, SimDecision{Before: len(r.out.Steps), What: what, Did: did})
	}

	return did, nil
}

/*
scenarioContext is what the decision model reads: the scenario, the
person, and the messages sent so far, oldest first.
*/
func (r *simRun) scenarioContext() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Scenario: %s\n\n%s", r.in.Scenario, profileOf(&run{people: []models.Contact{r.person}}))
	if len(r.out.Messages) == 0 {
		b.WriteString("\nNo message was sent to them yet.\n")
		return b.String()
	}
	b.WriteString("\nMessages sent to them so far:\n")
	for i, m := range r.out.Messages {
		fmt.Fprintf(&b, "%d. %s: %s\n%s\n", i+1, m.Kind, m.Subject, cut(m.Body, 600))
	}

	return b.String()
}

/*
engagementWords says what doing an engagement with a message is.
*/
func (r *simRun) engagementWords(e *Engagement) string {
	message := "the latest message"
	for _, m := range r.out.Messages {
		if m.NodeID == e.Node {
			message = "the message"
			if m.Subject != "" {
				message = fmt.Sprintf("the message %q", m.Subject)
			}
		}
	}
	what := "open " + message
	switch e.Action {
	case "clicked":
		what = "click a link in " + message
	case "replied":
		what = "reply to " + message
	}
	return what
}

/*
calledAside tells the model what the series calls a check.
*/
func calledAside(label string) string {
	if label == "" {
		return ""
	}

	return fmt.Sprintf(" (the series calls this check %q)", label)
}

/*
got reports whether the person got the message a check names; a check
that names none reads the latest message.
*/
func (r *simRun) got(node string) bool {
	if node == "" {
		return len(r.out.Messages) > 0
	}
	for _, m := range r.out.Messages {
		if m.NodeID == node {
			return true
		}
	}

	return false
}

/*
eventWords says what doing an event is, by the label and description
the workspace gave it when it has them.
*/
func (r *simRun) eventWords(ctx context.Context, event string) (what, aside string) {
	what = fmt.Sprintf("do the event %q in the product", event)
	_ = r.s.inWorkspace(ctx, r.ws, func(ctx context.Context, q *models.Queries) error {
		def, err := q.FindEventDefinitionByName(ctx, models.FindEventDefinitionByNameParams{
			WorkspaceID: r.ws, Name: event,
		})
		if err != nil || def.Label == "" {
			return nil //nolint:nilerr // an event nobody described reads by its name
		}
		what = fmt.Sprintf("do %q in the product", def.Label)
		aside = fmt.Sprintf(" (the event %s", event)
		if def.Description != nil && *def.Description != "" {
			aside += ": " + cut(*def.Description, 200)
		}
		aside += ")"
		return nil
	})

	return what, aside
}
