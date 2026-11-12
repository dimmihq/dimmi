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
| cannot be called from a workflow that runs in the background. Calls
| may never lead back to the caller, and a request through a connection
| needs that connection to exist.
|
*/

package workflows

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
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
	if err != nil {
		return nil, err
	}

	issues = append(issues, calls...)
	if len(calls) == 0 {
		cycle, err := w.cycle(ctx, q, wf, sets["workflow"])
		if err != nil {
			return nil, err
		}

		issues = append(issues, cycle...)
	}

	connections, err := w.connections(ctx, q, wf, refs.Connections)

	return append(issues, connections...), err
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
cycle refuses a definition whose calls lead back to itself through the
live reusable workflows, which would call each other until the depth
limit cut them off.
*/
func (w *Workflows) cycle(ctx context.Context, q *models.Queries, wf models.Workflow, called idSet,
) ([]workflowspec.Issue, error) {
	if len(called) == 0 {
		return nil, nil
	}

	rows, err := q.ListLiveWorkflowVersions(ctx, models.ListLiveWorkflowVersionsParams{
		WorkspaceID: wf.WorkspaceID, Trigger: workflowspec.TriggerReusable,
	})
	if err != nil {
		return nil, fmt.Errorf("list reusable workflows: %w", err)
	}

	edges := map[ids.UUID][]ids.UUID{}
	for _, row := range rows {
		spec, err := workflowspec.Parse(row.Spec)
		if err != nil {
			continue
		}

		set, _ := w.decodeAll("workflow", workflowspec.Compile(spec).References.Workflows)
		edges[row.WorkflowID] = values(set)
	}

	for public, id := range called {
		if reaches(edges, id, wf.ID, map[ids.UUID]bool{}) {
			return []workflowspec.Issue{{
				Code: workflowspec.CodeCallCycle, Field: public,
				Message: "This workflow ends up running the workflow that runs it.",
			}}, nil
		}
	}

	return nil, nil
}

/*
reaches reports whether the calls from one workflow lead to another.
*/
func reaches(edges map[ids.UUID][]ids.UUID, from, to ids.UUID, seen map[ids.UUID]bool) bool {
	if from == to {
		return true
	}

	if seen[from] {
		return false
	}

	seen[from] = true
	for _, next := range edges[from] {
		if reaches(edges, next, to, seen) {
			return true
		}
	}

	return false
}

/*
connections refuses requests through a connection the workspace no
longer has.
*/
func (w *Workflows) connections(ctx context.Context, q *models.Queries, wf models.Workflow, list []string,
) ([]workflowspec.Issue, error) {
	var issues []workflowspec.Issue

	for _, public := range list {
		id, err := w.IDs.Decode(string(models.PrefixAiActionConnection), public)
		if err != nil {
			issues = append(issues, missingIssue("connection", public))
			continue
		}

		conn, err := q.FindActionConnection(ctx, models.FindActionConnectionParams{
			WorkspaceID: wf.WorkspaceID, ID: id,
		})
		if errors.Is(err, database.ErrNotFound) || err == nil && conn.Kind != "http" {
			issues = append(issues, missingIssue("connection", public))
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("find connection: %w", err)
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
