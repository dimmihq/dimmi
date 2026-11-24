/*
|--------------------------------------------------------------------------
| Checking Workflow And Series Proposals
|--------------------------------------------------------------------------
|
| A proposal is applied to the working copy, or to the suggestion the
| teammate has open on it, and the result is checked the way publishing
| checks it. Only problems the edit adds are refused; ones the working
| copy already had are left to the teammate, and said.
|
*/

package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
automationOp checks a workflow or series operation by its kind.
*/
func (r resolver) automationOp(ctx context.Context, in opInput) (Operation, error) {
	if in.auto == nil {
		return Operation{}, fmt.Errorf("%w: suggest workflow and series changes with %s, %s, %s or %s",
			errBadArguments, toolWorkflowEdit, toolWorkflowNew, toolSeriesEdit, toolSeriesNew)
	}
	switch in.Kind {
	case KindWorkflowSet:
		return r.workflowOp(ctx, *in.auto)
	case KindWorkflowNew:
		return r.newWorkflowOp(ctx, *in.auto)
	case KindSeriesSet:
		return r.seriesOp(ctx, *in.auto)
	default:
		return r.newSeriesOp(ctx, *in.auto)
	}
}

/*
workflowOp checks an edit of a workflow's working copy.
*/
func (r resolver) workflowOp(ctx context.Context, a automationInput) (Operation, error) {
	op := Operation{Kind: KindWorkflowSet}
	wf, base, err := r.workflowBase(ctx, a.ID)
	if err != nil {
		return op, err
	}
	next, err := nextWorkflow(base, a)
	if err != nil {
		return op, err
	}
	raw, report, err := r.checkedEdit(ctx, wf, base, next)
	if err != nil {
		return op, err
	}
	op.Workflow, op.Definition = &wf.ID, raw
	op.Label = fmt.Sprintf("Edit the working copy of the workflow “%s”", wf.Name)
	op.Note = "\nThe workflow would read:\n" + workflowOutline(next) + reportText(report)
	return op, nil
}

/*
workflowBase reads the workflow an edit is about and the working copy it
builds on: the open suggestion's, else the saved one.
*/
func (r resolver) workflowBase(ctx context.Context, public string) (models.Workflow, workflowspec.Spec, error) {
	id, err := r.e.IDs.Decode(string(models.PrefixWorkflow), strings.TrimSpace(public))
	if err != nil {
		return models.Workflow{}, workflowspec.Spec{}, fmt.Errorf("%w: there is no workflow %q; \"workflow_id\" "+
			"is the id list_workflows shows (wf_...)", errBadArguments, public)
	}
	if !r.s.m.Can("workflows:manage") {
		return models.Workflow{}, workflowspec.Spec{}, exceptions.ErrForbidden
	}
	wf, err := r.e.Services.Workflows.Show(ctx, r.s.m, id)
	if err != nil {
		return wf, workflowspec.Spec{}, err
	}
	if wf.DeletedAt != nil {
		return wf, workflowspec.Spec{}, fmt.Errorf("%w: the workflow %q is archived; unarchive it first",
			errBadArguments, wf.Name)
	}
	base, err := workflowspec.Parse(wf.Draft)
	if suggested, ok := r.s.suggestedWorkflow(id); ok {
		base = suggested
	}
	return wf, base, err
}

/*
checkedEdit is the canonical working copy after an edit, and what
publishing would find in it; an edit that changes the trigger, changes
nothing or adds an error publishing would refuse is refused.
*/
func (r resolver) checkedEdit(ctx context.Context, wf models.Workflow, base, next workflowspec.Spec,
) (json.RawMessage, workflowspec.Report, error) {
	if next.Trigger != wf.Trigger {
		return nil, workflowspec.Report{}, fmt.Errorf("%w: a workflow's trigger never changes; this one starts "+
			"on %s. To start on %s, draft a new workflow with %s", errBadArguments, wf.Trigger, next.Trigger,
			toolWorkflowNew)
	}
	raw, err := canonicalWorkflow(&next)
	if err != nil {
		return nil, workflowspec.Report{}, err
	}
	if was, _ := workflowspec.Canonical(base); bytes.Equal(was, raw) {
		return nil, workflowspec.Report{}, fmt.Errorf("%w: the edit leaves the workflow exactly as it is; do not "+
			"send it again", errBadArguments)
	}
	before, err := r.e.Services.Workflows.CheckDraft(ctx, r.s.m, wf, base)
	if err != nil {
		return nil, before, err
	}
	after, err := r.e.Services.Workflows.CheckDraft(ctx, r.s.m, wf, next)
	if err != nil {
		return nil, after, err
	}
	if added := freshIssues(before.Errors, after.Errors); len(added) > 0 {
		return nil, after, refusedWorkflow(added, next)
	}
	return raw, after, nil
}

/*
newWorkflowOp checks a new draft workflow.
*/
func (r resolver) newWorkflowOp(ctx context.Context, a automationInput) (Operation, error) {
	op := Operation{Kind: KindWorkflowNew}
	if !r.s.m.Can("workflows:manage") {
		return op, exceptions.ErrForbidden
	}
	name, err := r.freeWorkflowName(ctx, a.Name)
	if err != nil {
		return op, err
	}
	spec, err := newWorkflowSpec(a)
	if err != nil {
		return op, err
	}
	raw, err := canonicalWorkflow(&spec)
	if err != nil {
		return op, err
	}
	report, err := r.e.Services.Workflows.CheckDraft(ctx, r.s.m, models.Workflow{Trigger: spec.Trigger}, spec)
	if err != nil {
		return op, err
	}
	if len(report.Errors) > 0 {
		return op, refusedWorkflow(report.Errors, spec)
	}
	op.Definition = rawJSON(newWorkflowDraft{
		Name: name, Description: strings.TrimSpace(a.Description), Trigger: spec.Trigger, Spec: raw,
	})
	op.Label = fmt.Sprintf("Draft a new workflow “%s”", name)
	op.Note = "\nThe workflow would read:\n" + workflowOutline(spec) + reportText(report)
	return op, nil
}

/*
freeWorkflowName is a new workflow's name, refused when it is empty,
too long or taken.
*/
func (r resolver) freeWorkflowName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return "", fmt.Errorf("%w: give the workflow a name of 1 to 80 characters", errBadArguments)
	}
	taken, err := r.e.Services.Workflows.NameTaken(ctx, r.s.m, name)
	if err != nil {
		return "", err
	}
	if taken {
		return "", fmt.Errorf("%w: another workflow is called %q; pick another name", errBadArguments, name)
	}
	return name, nil
}

/*
newWorkflowSpec reads a new workflow's definition, with its trigger and
the trigger's defaults.
*/
func newWorkflowSpec(a automationInput) (workflowspec.Spec, error) {
	if len(a.Spec) == 0 {
		return workflowspec.Spec{}, fmt.Errorf("%w: give the whole definition as spec", errBadArguments)
	}
	spec, err := workflowspec.Parse(a.Spec)
	if err != nil {
		return spec, specRefusal(err)
	}
	trigger := strings.TrimSpace(a.Trigger)
	spec.Trigger = either(spec.Trigger, trigger)
	t, ok := workflowspec.FindTrigger(spec.Trigger)
	if !ok || (trigger != "" && trigger != spec.Trigger) {
		return spec, fmt.Errorf("%w: give one trigger key that exists, the same in trigger and spec; "+
			"get_workflow_reference lists them", errBadArguments)
	}
	if spec.Schedule.Mode == "" {
		spec.Schedule.Mode = "any"
	}
	if spec.Reentry.Mode == "" {
		spec.Reentry.Mode = t.Default
	}
	return spec, nil
}

/*
nextWorkflow is the working copy after the proposal: the whole spec it
sent, or the edits applied to the base.
*/
func nextWorkflow(base workflowspec.Spec, a automationInput) (workflowspec.Spec, error) {
	switch {
	case len(a.Spec) > 0 && len(a.Workflow) > 0:
		return base, fmt.Errorf("%w: send either edits or a whole spec, not both", errBadArguments)
	case len(a.Spec) > 0:
		next, err := workflowspec.Parse(a.Spec)
		if err != nil {
			return base, specRefusal(err)
		}
		next.Trigger = either(next.Trigger, base.Trigger)
		return next, nil
	case len(a.Workflow) > 0:
		return applyWorkflowEdits(base, a.Workflow)
	default:
		return base, fmt.Errorf("%w: \"edits\" is missing; send the changes as edits", errBadArguments)
	}
}

/*
canonicalWorkflow gives new steps and paths their ids and returns the
canonical JSON the working copy is stored as.
*/
func canonicalWorkflow(spec *workflowspec.Spec) (json.RawMessage, error) {
	if spec.Schema == 0 {
		spec.Schema = workflowspec.SchemaVersion
	}
	workflowspec.AssignIDs(spec)
	raw, err := workflowspec.Canonical(*spec)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errBadArguments, err.Error())
	}
	if len(raw) > workflowspec.MaxBytes {
		return nil, fmt.Errorf("%w: the workflow would be over 256 KB", errBadArguments)
	}
	return raw, nil
}

/*
specRefusal says why a spec could not be read.
*/
func specRefusal(err error) error {
	var bad *workflowspec.InvalidError
	if errors.As(err, &bad) && len(bad.Errors) > 0 {
		return fmt.Errorf("%w: %s", errBadArguments, bad.Errors[0].Message)
	}
	return fmt.Errorf("%w: %s", errBadArguments, err.Error())
}

/*
refusedWorkflow refuses an edit with every problem it adds and the
workflow as it would have read, so the model can fix them in one go.
*/
func refusedWorkflow(issues []workflowspec.Issue, spec workflowspec.Spec) error {
	return fmt.Errorf("%w: publishing would refuse this, so nothing was suggested. Fix each problem below in "+
		"the same call and send it again:\n%s\nThe workflow would have read:\n%s", errBadArguments,
		issueLines(issues), workflowOutline(spec))
}

/*
freshIssues are the issues of after that before did not have.
*/
func freshIssues(before, after []workflowspec.Issue) []workflowspec.Issue {
	had := map[workflowspec.Issue]bool{}
	for _, i := range before {
		had[i] = true
	}
	var out []workflowspec.Issue
	for _, i := range after {
		if !had[i] {
			out = append(out, i)
		}
	}
	return out
}

/*
suggestedWorkflow is a workflow's working copy as the open suggestion
would leave it, which further edits build on.
*/
func (s *session) suggestedWorkflow(id ids.UUID) (workflowspec.Spec, bool) {
	if s.viewed == nil {
		return workflowspec.Spec{}, false
	}
	for _, op := range Operations(s.viewed.Operations) {
		if op.Kind == KindWorkflowSet && op.Workflow != nil && *op.Workflow == id {
			spec, err := workflowspec.Parse(op.Definition)
			return spec, err == nil
		}
	}
	return workflowspec.Spec{}, false
}

/*
automationOutline is how a workflow or series operation reads as
suggested, as the outline the model edits by.
*/
func automationOutline(op Operation) (string, bool) {
	switch op.Kind {
	case KindWorkflowSet:
		spec, err := workflowspec.Parse(op.Definition)
		return workflowOutline(spec), err == nil
	case KindSeriesSet:
		spec, err := readSeries(op.Definition)
		return seriesOutline(spec), err == nil
	}
	return "", false
}
