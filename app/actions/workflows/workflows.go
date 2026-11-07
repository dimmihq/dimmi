/*
|--------------------------------------------------------------------------
| Workflows
|--------------------------------------------------------------------------
|
| A workflow is created with its trigger, which never changes, and a
| working copy people edit freely: saving a draft never changes what
| runs. Every save carries the lock version it started from, so two
| editors cannot overwrite each other without noticing. Drafts may be
| incomplete; only publishing insists on a definition that can run.
|
| Every member may read workflows, because they need to understand why
| a conversation went where it went. Only those who may manage
| workflows change them.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
Failures of managing workflows.
*/
var (
	ErrWorkflowNotFound = fault.New(fault.NotFound, "workflow_not_found", "No workflow of this workspace has this id.")
	ErrNameTaken        = fault.New(fault.Conflict, "name_taken",
		"Another workflow already has this name.").OnField("name")
	ErrNameInvalid  = fault.New(fault.Invalid, "name_invalid", "Names are 1 to 80 characters.").OnField("name")
	ErrEditConflict = fault.New(fault.Conflict, "edit_conflict",
		"Someone else saved this workflow since you opened it. Reload to see their changes.")
	ErrTriggerImmutable = fault.New(fault.Invalid, "trigger_immutable",
		"A workflow's trigger never changes. Duplicate it into another trigger instead.").OnField("trigger")
	ErrUnknownTrigger = fault.New(fault.Invalid, "unknown_trigger", "No trigger has this name.").OnField("trigger")
	ErrArchived       = fault.New(fault.Conflict, "workflow_archived", "Unarchive the workflow first.")
)

/*
Workflows manages a workspace's workflows.
*/
type Workflows struct {
	DB    *database.DB
	Clock clock.Clock
	IDs   *ids.Registry
	Live  Changed
}

/*
Changed is told when the set of live workflows of a workspace changed,
so cached copies of it can be dropped.
*/
type Changed interface {
	/*
		WorkflowsChanged drops what is cached about the workspace's live
		workflows.
	*/
	WorkflowsChanged(ctx context.Context, workspace ids.UUID)
}

/*
CreateInput is a workflow to create: a name, a trigger, and optionally a
definition to start from, such as a template or an export.
*/
type CreateInput struct {
	Membership  authz.Membership
	Name        string
	Description *string
	Trigger     string
	Spec        json.RawMessage
}

/*
UpdateInput is a save of a workflow's working copy. Nil fields stay as
they are. LockVersion is the version the editor started from; zero
skips the check, for API clients that want the last write to win.
*/
type UpdateInput struct {
	Membership  authz.Membership
	ID          ids.UUID
	LockVersion int32
	Name        *string
	Description *string
	Spec        json.RawMessage
	TestOnTest  *bool
}

/*
List returns the workspace's workflows in list order.
*/
func (w *Workflows) List(ctx context.Context, m authz.Membership, archived bool) ([]models.Workflow, error) {
	if !m.Can("workflows:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.Workflow
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListWorkflows(ctx, models.ListWorkflowsParams{
			WorkspaceID: m.Workspace.ID, Archived: archived,
		})

		return err
	})

	return out, err
}

/*
Show returns one workflow.
*/
func (w *Workflows) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.Workflow, error) {
	if !m.Can("workflows:read") {
		return models.Workflow{}, exceptions.ErrForbidden
	}

	var out models.Workflow
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = find(ctx, models.New(tx), m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
Create stores a new draft workflow at the end of the list.
*/
func (w *Workflows) Create(ctx context.Context, in CreateInput) (models.Workflow, error) {
	if !in.Membership.Can("workflows:manage") {
		return models.Workflow{}, exceptions.ErrForbidden
	}

	name, err := cleanName(in.Name)
	if err != nil {
		return models.Workflow{}, err
	}

	spec, err := startingSpec(in)
	if err != nil {
		return models.Workflow{}, err
	}

	d, err := encodeDraft(spec)
	if err != nil {
		return models.Workflow{}, err
	}

	ws, now := in.Membership.Workspace.ID, w.Clock.Now()

	var out models.Workflow
	err = w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		free := models.WorkflowNameTakenParams{WorkspaceID: ws, Name: name, ID: ids.Nil()}
		if err := nameFree(ctx, q, free); err != nil {
			return err
		}

		position, err := q.NextWorkflowPosition(ctx, ws)
		if err != nil {
			return fmt.Errorf("place workflow: %w", err)
		}

		out, err = q.CreateWorkflow(ctx, models.CreateWorkflowParams{
			ID: ids.NewV7At(now), WorkspaceID: ws, Name: name, Description: clean(in.Description, 500),
			Trigger: spec.Trigger, Position: position, Draft: d.raw, DraftHash: d.hash, Kind: d.kind,
			MemberID: &in.Membership.Member.ID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("create workflow: %w", err)
		}

		return nil
	})

	return out, err
}

/*
Update saves the working copy. The trigger in a definition must match
the workflow's own.
*/
func (w *Workflows) Update(ctx context.Context, in UpdateInput) (models.Workflow, error) {
	if !in.Membership.Can("workflows:manage") {
		return models.Workflow{}, exceptions.ErrForbidden
	}

	ws, now := in.Membership.Workspace.ID, w.Clock.Now()

	var out models.Workflow
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lock(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}

		params, err := updated(current, in)
		if err != nil {
			return err
		}

		free := models.WorkflowNameTakenParams{WorkspaceID: ws, Name: params.Name, ID: current.ID}
		if err := nameFree(ctx, q, free); err != nil {
			return err
		}

		params.MemberID, params.Now = &in.Membership.Member.ID, now
		out, err = q.UpdateWorkflowDraft(ctx, params)
		if err != nil {
			return fmt.Errorf("save workflow: %w", err)
		}

		return nil
	})

	return out, err
}

/*
updated writes the input over the current workflow.
*/
func updated(current models.Workflow, in UpdateInput) (models.UpdateWorkflowDraftParams, error) {
	p := models.UpdateWorkflowDraftParams{
		Name: current.Name, Description: current.Description, Draft: current.Draft, DraftHash: current.DraftHash,
		Kind: current.Kind, TestOnTestConversations: current.TestOnTestConversations,
		WorkspaceID: current.WorkspaceID, ID: current.ID, LockVersion: current.LockVersion,
	}

	if in.LockVersion != 0 && in.LockVersion != current.LockVersion {
		return p, ErrEditConflict
	}

	if in.Name != nil {
		name, err := cleanName(*in.Name)
		if err != nil {
			return p, err
		}

		p.Name = name
	}

	if in.Description != nil {
		p.Description = clean(in.Description, 500)
	}

	if in.TestOnTest != nil {
		p.TestOnTestConversations = *in.TestOnTest
	}

	if len(in.Spec) == 0 {
		return p, nil
	}

	return withSpec(p, current.Trigger, in.Spec)
}

/*
withSpec reads a new working copy into the update.
*/
func withSpec(p models.UpdateWorkflowDraftParams, trigger string, raw json.RawMessage,
) (models.UpdateWorkflowDraftParams, error) {
	spec, err := workflowspec.Parse(raw)
	if err != nil {
		return p, invalid(err)
	}

	if spec.Trigger == "" {
		spec.Trigger = trigger
	}

	if spec.Trigger != trigger {
		return p, ErrTriggerImmutable
	}

	d, err := encodeDraft(spec)
	if err != nil {
		return p, err
	}

	p.Draft, p.DraftHash, p.Kind = d.raw, d.hash, d.kind

	return p, nil
}

/*
find returns a workflow of the workspace.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Workflow, error) {
	wf, err := q.FindWorkflow(ctx, models.FindWorkflowParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return wf, ErrWorkflowNotFound
	}

	if err != nil {
		return wf, fmt.Errorf("find workflow: %w", err)
	}

	return wf, nil
}

/*
lock returns a live workflow of the workspace and holds it.
*/
func lock(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Workflow, error) {
	wf, err := q.LockWorkflow(ctx, models.LockWorkflowParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return wf, ErrWorkflowNotFound
	}

	if err != nil {
		return wf, fmt.Errorf("lock workflow: %w", err)
	}

	if wf.DeletedAt != nil {
		return wf, ErrArchived
	}

	return wf, nil
}

/*
nameFree refuses a name another workflow already has.
*/
func nameFree(ctx context.Context, q *models.Queries, name models.WorkflowNameTakenParams) error {
	taken, err := q.WorkflowNameTaken(ctx, name)
	if err != nil {
		return fmt.Errorf("check workflow name: %w", err)
	}

	if taken {
		return ErrNameTaken
	}

	return nil
}

/*
cleanName trims a name and checks its length.
*/
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return "", ErrNameInvalid
	}

	return name, nil
}

/*
clean trims optional text and cuts it to the limit; empty becomes nil.
*/
func clean(s *string, limit int) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}

	out := []rune(strings.TrimSpace(*s))
	if len(out) > limit {
		out = out[:limit]
	}

	text := string(out)

	return &text
}
