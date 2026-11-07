/*
|--------------------------------------------------------------------------
| Workflow Versions
|--------------------------------------------------------------------------
|
| Every publish leaves a version behind, newest first in the history.
| Restoring one copies it into the working copy; publishing it again
| makes a new version that remembers where it came from, so the history
| only ever grows. A restored version that names a team or tag deleted
| since is refused on publish, never silently emptied.
|
*/

package workflows

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxVersionsListed bounds the history returned at once.
*/
const maxVersionsListed = 100

/*
RestoreInput restores a version into the working copy, and publishes it
when Publish is set.
*/
type RestoreInput struct {
	Membership          authz.Membership
	ID                  ids.UUID
	VersionID           ids.UUID
	Publish             bool
	AcknowledgeWarnings bool
	Via                 string
}

/*
Versions returns a workflow's versions, newest first.
*/
func (w *Workflows) Versions(ctx context.Context, m authz.Membership, id ids.UUID) ([]models.WorkflowVersion, error) {
	if !m.Can("workflows:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.WorkflowVersion
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		var err error
		out, err = q.ListWorkflowVersions(ctx, models.ListWorkflowVersionsParams{
			WorkspaceID: m.Workspace.ID, WorkflowID: id, RowLimit: maxVersionsListed,
		})

		return err
	})

	return out, err
}

/*
Restore copies a version into the working copy.
*/
func (w *Workflows) Restore(ctx context.Context, in RestoreInput) (Published, error) {
	if !in.Membership.Can("workflows:manage") {
		return Published{}, exceptions.ErrForbidden
	}

	var out Published
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, version, err := w.restoreLocked(ctx, q, in)
		if err != nil {
			return err
		}

		out.Workflow = wf
		if !in.Publish {
			return nil
		}

		out, err = w.publishLocked(ctx, q, PublishInput{
			Membership: in.Membership, ID: in.ID, AcknowledgeWarnings: in.AcknowledgeWarnings, Via: in.Via,
			restoredFrom: &version.ID,
		})

		return err
	})
	if err == nil && in.Publish {
		w.changed(ctx, in.Membership.Workspace.ID)
	}

	return out, err
}

/*
restoreLocked writes the version's definition over the working copy.
*/
func (w *Workflows) restoreLocked(ctx context.Context, q *models.Queries, in RestoreInput,
) (models.Workflow, models.WorkflowVersion, error) {
	ws := in.Membership.Workspace.ID
	wf, err := lock(ctx, q, ws, in.ID)
	if err != nil {
		return wf, models.WorkflowVersion{}, err
	}

	version, err := q.FindWorkflowVersion(ctx, models.FindWorkflowVersionParams{WorkspaceID: ws, ID: in.VersionID})
	if errors.Is(err, database.ErrNotFound) || err == nil && version.WorkflowID != wf.ID {
		return wf, version, ErrVersionNotFound
	}

	if err != nil {
		return wf, version, fmt.Errorf("find workflow version: %w", err)
	}

	wf, err = q.UpdateWorkflowDraft(ctx, models.UpdateWorkflowDraftParams{
		Name: wf.Name, Description: wf.Description, Draft: version.Spec, DraftHash: version.SpecHash,
		Kind: version.Kind, TestOnTestConversations: wf.TestOnTestConversations, MemberID: &in.Membership.Member.ID,
		Now: w.Clock.Now(), WorkspaceID: ws, ID: wf.ID, LockVersion: wf.LockVersion,
	})
	if err != nil {
		return wf, version, fmt.Errorf("restore workflow version: %w", err)
	}

	return wf, version, nil
}
