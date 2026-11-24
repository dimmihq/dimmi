/*
|--------------------------------------------------------------------------
| Dry Run Waits And Calls
|--------------------------------------------------------------------------
|
| A wait in a dry run asks the driver whether the customer writes before
| the time is up, when the customer may interrupt it: a message takes
| the interrupted path, as a run's does, and silence lets the time pass
| at once. A run workflow step walks the live reusable workflow it
| names inline, nested one level deeper, on the same variables and the
| same copy of the conversation, then goes back to the caller, as a
| called run gives its variables back. Calls nest as deep as a run's.
|
*/

package workflows

import (
	"context"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
dryCall is the workflow a dry run walks inside a call: its public id
and name.
*/
type dryCall struct {
	id   string
	name string
}

/*
wait lets the customer write before the time is up, when they may
interrupt it, and otherwise lets the time pass at once.
*/
func (d *dry) wait(ctx context.Context, s workflowspec.Step) (outcome, error) {
	w := settings[workflowspec.WaitSettings](s)
	passed := outcome{outcome: outcomeElapsed, status: "skipped", detail: map[string]any{"wait": "skipped"}}
	if len(w.InterruptOn) > 0 && !slices.Contains(w.InterruptOn, interruptCustomer) {
		return passed, nil
	}
	answer, err := d.in.Driver.Customer(ctx, DryAsk{
		StepID: s.ID, Kind: s.Kind, CanWait: true, Waiting: waitWords(w),
	})
	if err != nil || answer.Silent || strings.TrimSpace(answer.Text) == "" {
		return passed, err
	}
	out := fixedOrEnd(s, workflowspec.OutcomeInterrupted, EndInterrupted)
	out.outcome = workflowspec.OutcomeInterrupted

	return withDetail(withDetail(out, "by", interruptCustomer), "message", answer.Text), nil
}

/*
waitWords says how long a wait waits, for the customer's model.
*/
func waitWords(w workflowspec.WaitSettings) string {
	switch {
	case w.Until != nil && w.Until.Kind == untilOpen:
		return "until the office opens"
	case w.Until != nil:
		return "until " + w.Until.Time
	case w.Count == "business":
		return w.For + " of office hours"
	}

	return w.For
}

/*
call walks the workflow a run workflow step names inside the dry run,
then goes on along the caller's path. It records its own step: once
when the call starts, so whoever watches sees it before the called
steps, and again with how the called workflow ended.
*/
func (d *dry) call(ctx context.Context, s workflowspec.Step) (outcome, error) {
	r := settings[workflowspec.RunWorkflowSettings](s)
	nothing := func(reason string) (outcome, error) {
		out := calledNothing(reason, r.WorkflowID)
		d.record(ctx, s, out)
		return out, nil
	}
	if d.depth+1 > MaxCallDepth {
		return nothing("depth_limit")
	}
	c, ok, err := d.callee(ctx, r.WorkflowID)
	switch {
	case err != nil:
		return outcome{}, err
	case !ok:
		return nothing(outcomeMissingRef)
	case c.kind == workflowspec.KindCustomerFacing &&
		workflowspec.WorkflowKind(d.spec) != workflowspec.KindCustomerFacing:
		return nothing("customer_facing_from_background")
	}
	detail := map[string]any{"workflow_id": r.WorkflowID, "workflow": c.name}
	d.out.Actions = append(d.out.Actions, DryAction{
		StepID: s.ID, Kind: s.Kind, Summary: "Would run the workflow " + c.name, Detail: detail,
	})
	at := len(d.out.Steps)
	d.record(ctx, s, outcome{outcome: "calling", acted: true, detail: detail})

	index, cursor, depth, called := d.index, d.cursor, d.depth, d.called
	d.index, d.cursor, d.depth = c.index, c.index.First(), depth+1
	d.called = &dryCall{id: r.WorkflowID, name: c.name}
	end, err := d.walkPath(ctx)
	d.index, d.cursor, d.depth, d.called = index, cursor, depth, called
	if err != nil {
		return outcome{}, err
	}

	out := outcome{outcome: "completed", acted: true, detail: map[string]any{
		"workflow_id": r.WorkflowID, "workflow": c.name, "child_end": end,
	}}
	d.out.Steps[at] = d.traced(s, out)

	return out, nil
}

/*
callee returns the live reusable workflow with the public id, never
the workflow the dry run walks.
*/
func (d *dry) callee(ctx context.Context, public string) (candidate, bool, error) {
	id, ok := d.decode(models.PrefixWorkflow, public)
	if !ok || id == d.in.WorkflowID {
		return candidate{}, false, nil
	}
	var found candidate
	hit := false
	err := d.read(ctx, func(ctx context.Context, q *models.Queries) error {
		live, err := d.e.candidates(ctx, q, d.in.WorkspaceID, workflowspec.TriggerReusable)
		for _, c := range live {
			if c.workflowID == id {
				found, hit = c, true
			}
		}
		return err
	})
	if err != nil || !hit {
		return candidate{}, false, err
	}

	return found, true, nil
}
