/*
|--------------------------------------------------------------------------
| Workflow References
|--------------------------------------------------------------------------
|
| A definition names teams, members, tags, schedules, saved audiences and
| other workflows by public id. Before it goes live every one of them must exist and be
| usable: a deactivated member or a deleted tag would make the step
| silently do nothing. A called workflow must be a live reusable
| workflow, never the caller itself, and one that speaks to customers
| cannot be called from a workflow that runs in the background.
|
*/

package workflows

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
idSet is the decoded ids of one kind of reference, by public id.
*/
type idSet map[string]ids.UUID

/*
references returns an error issue for every reference that is malformed
or names nothing usable.
*/
func (w *Workflows) references(
	ctx context.Context, q *models.Queries, wf models.Workflow, spec workflowspec.Spec,
) ([]workflowspec.Issue, error) {
	refs := workflowspec.Compile(spec).References
	var issues []workflowspec.Issue

	sets := map[string]idSet{}
	for kind, list := range map[string][]string{
		"team": refs.Teams, "member": refs.Members, "tag": refs.Tags, "schedule": refs.Schedules,
		"workflow": refs.Workflows, "audience": refs.Audiences,
	} {
		set, bad := w.decodeAll(kind, list)
		sets[kind] = set
		issues = append(issues, bad...)
	}

	missing, err := q.MissingWorkflowReferences(ctx, models.MissingWorkflowReferencesParams{
		WorkspaceID: wf.WorkspaceID, TeamIds: values(sets["team"]), MemberIds: values(sets["member"]),
		TagIds: values(sets["tag"]), ScheduleIds: values(sets["schedule"]), AudienceIds: values(sets["audience"]),
	})
	if err != nil {
		return nil, fmt.Errorf("check workflow references: %w", err)
	}

	for _, m := range missing {
		issues = append(issues, missingIssue(m.Kind, publicOf(sets[m.Kind], m.ID)))
	}

	calls, err := w.calls(ctx, q, wf, sets["workflow"])

	return append(issues, calls...), err
}

/*
calls checks the reusable workflows a definition runs.
*/
func (w *Workflows) calls(ctx context.Context, q *models.Queries, wf models.Workflow, called idSet,
) ([]workflowspec.Issue, error) {
	if len(called) == 0 {
		return nil, nil
	}

	live, err := q.LiveReusableWorkflows(ctx, models.LiveReusableWorkflowsParams{
		WorkspaceID: wf.WorkspaceID, Ids: values(called),
	})
	if err != nil {
		return nil, fmt.Errorf("check called workflows: %w", err)
	}

	background := wf.Kind == workflowspec.KindBackground
	var issues []workflowspec.Issue

	for public, id := range called {
		i := slices.IndexFunc(live, func(r models.LiveReusableWorkflowsRow) bool { return r.ID == id })
		switch {
		case id == wf.ID:
			issues = append(issues, workflowspec.Issue{
				Code: workflowspec.CodeSelfCall, Field: public, Message: "A workflow cannot run itself.",
			})
		case i < 0:
			issues = append(issues, missingIssue("workflow", public))
		case background && live[i].Kind == workflowspec.KindCustomerFacing:
			issues = append(issues, workflowspec.Issue{
				Code: workflowspec.CodeCallFromBackground, Field: public,
				Message: "This workflow talks to customers, so a background workflow cannot run it.",
			})
		}
	}

	return issues, nil
}

/*
decodeAll decodes public ids of one kind, reporting malformed ones.
*/
func (w *Workflows) decodeAll(kind string, list []string) (idSet, []workflowspec.Issue) {
	prefix := map[string]models.Prefix{
		"team": models.PrefixTeam, "member": models.PrefixMember, "tag": models.PrefixTag,
		"schedule": models.PrefixSchedule, "workflow": models.PrefixWorkflow, "audience": models.PrefixAudience,
	}[kind]

	set := idSet{}
	var bad []workflowspec.Issue

	for _, public := range list {
		id, err := w.IDs.Decode(string(prefix), public)
		if err != nil {
			bad = append(bad, missingIssue(kind, public))
			continue
		}

		set[public] = id
	}

	return set, bad
}

/*
missingIssue is the error of a reference that names nothing usable.
*/
func missingIssue(kind, public string) workflowspec.Issue {
	return workflowspec.Issue{
		Code: workflowspec.CodeReferenceMissing, Field: public,
		Message: fmt.Sprintf("No usable %s has the id %s.", kind, public),
	}
}

/*
values returns a set's ids.
*/
func values(set idSet) []ids.UUID {
	out := make([]ids.UUID, 0, len(set))
	for _, id := range set {
		out = append(out, id)
	}

	return out
}

/*
publicOf returns the public id a decoded id came from.
*/
func publicOf(set idSet, id ids.UUID) string {
	for public, decoded := range set {
		if decoded == id {
			return public
		}
	}

	return id.String()
}
