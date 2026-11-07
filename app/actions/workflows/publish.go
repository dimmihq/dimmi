/*
|--------------------------------------------------------------------------
| Publishing Workflows
|--------------------------------------------------------------------------
|
| Publishing freezes the working copy into a new version and makes it
| live. Conversations already running keep the version they started on;
| new events use the new one. A definition goes live only when it can
| run: every error is refused, every referenced team, member, tag and
| schedule must exist, and warnings must be acknowledged, because each
| describes customers likely to end up somewhere nobody meant.
|
| Rolling back copies an old version into the working copy, and can
| publish it as a new version at once. Nothing is ever rewritten.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
Failures of publishing.
*/
var (
	ErrNothingToPublish = fault.New(fault.Conflict, "nothing_to_publish",
		"The live version already matches the working copy.")
	ErrDraftChanged = fault.New(fault.Conflict, "draft_changed",
		"The working copy changed since you reviewed it. Review it again before publishing.")
	ErrVersionNotFound = fault.New(fault.NotFound, "workflow_version_not_found",
		"This workflow has no version with this id.")
)

/*
PublishInput publishes a workflow's working copy. DraftHash, when set,
must still match the working copy, so what goes live is what the
publisher reviewed. Via names how the publisher acted, such as session
or cli.
*/
type PublishInput struct {
	Membership          authz.Membership
	ID                  ids.UUID
	Note                *string
	AcknowledgeWarnings bool
	DraftHash           string
	Via                 string
	restoredFrom        *ids.UUID
}

/*
Published is the result of a publish: the workflow and its new version.
*/
type Published struct {
	Workflow models.Workflow
	Version  models.WorkflowVersion
}

/*
Publish makes the working copy the live version.
*/
func (w *Workflows) Publish(ctx context.Context, in PublishInput) (Published, error) {
	if !in.Membership.Can("workflows:manage") {
		return Published{}, exceptions.ErrForbidden
	}

	var out Published
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = w.publishLocked(ctx, models.New(tx), in)

		return err
	})
	if err != nil {
		return out, err
	}

	w.changed(ctx, in.Membership.Workspace.ID)

	return out, nil
}

/*
publishLocked checks and freezes the working copy inside a transaction.
*/
func (w *Workflows) publishLocked(ctx context.Context, q *models.Queries, in PublishInput) (Published, error) {
	ws, now := in.Membership.Workspace.ID, w.Clock.Now()
	wf, err := lock(ctx, q, ws, in.ID)
	if err != nil {
		return Published{}, err
	}

	if in.DraftHash != "" && in.DraftHash != wf.DraftHash {
		return Published{}, ErrDraftChanged
	}

	if err := w.nothingNew(ctx, q, wf); err != nil {
		return Published{}, err
	}

	spec, err := w.ready(ctx, q, wf, in)
	if err != nil {
		return Published{}, err
	}

	compiled, err := json.Marshal(workflowspec.Compile(spec))
	if err != nil {
		return Published{}, fmt.Errorf("compile workflow: %w", err)
	}

	number, err := q.NextWorkflowVersionNumber(ctx, models.NextWorkflowVersionNumberParams{
		WorkspaceID: ws, WorkflowID: wf.ID,
	})
	if err != nil {
		return Published{}, fmt.Errorf("number workflow version: %w", err)
	}

	version, err := q.CreateWorkflowVersion(ctx, models.CreateWorkflowVersionParams{
		ID: ids.NewV7At(now), WorkspaceID: ws, WorkflowID: wf.ID, Number: number, Spec: wf.Draft,
		SpecHash: wf.DraftHash, Kind: wf.Kind, Compiled: compiled, Note: clean(in.Note, 280),
		MemberID: &in.Membership.Member.ID, PublishedVia: via(in.Via), RestoredFromVersionID: in.restoredFrom, Now: now,
	})
	if err != nil {
		return Published{}, fmt.Errorf("create workflow version: %w", err)
	}

	live, err := q.SetWorkflowLive(ctx, models.SetWorkflowLiveParams{
		VersionID: &version.ID, LiveHash: &version.SpecHash, Now: now, WorkspaceID: ws, ID: wf.ID,
	})
	if err != nil {
		return Published{}, fmt.Errorf("set workflow live: %w", err)
	}

	return Published{Workflow: live, Version: version}, nil
}

/*
nothingNew refuses a publish when the live version already holds the
working copy and the workflow is live.
*/
func (w *Workflows) nothingNew(ctx context.Context, q *models.Queries, wf models.Workflow) error {
	if wf.LiveVersionID == nil || wf.Status != "live" {
		return nil
	}

	live, err := q.FindWorkflowVersion(ctx, models.FindWorkflowVersionParams{
		WorkspaceID: wf.WorkspaceID, ID: *wf.LiveVersionID,
	})
	if err != nil {
		return fmt.Errorf("find live version: %w", err)
	}

	if live.SpecHash == wf.DraftHash {
		return ErrNothingToPublish
	}

	return nil
}

/*
ready checks the definition can run: no errors, every reference
present, warnings acknowledged.
*/
func (w *Workflows) ready(ctx context.Context, q *models.Queries, wf models.Workflow, in PublishInput,
) (workflowspec.Spec, error) {
	spec, err := decodeDraft(wf.Draft)
	if err != nil {
		return spec, err
	}

	report := workflowspec.Check(spec)

	missing, err := w.references(ctx, q, wf, spec)
	if err != nil {
		return spec, err
	}

	report.Errors = append(report.Errors, missing...)
	if !report.OK() {
		return spec, invalid(report.Err())
	}

	if len(report.Warnings) > 0 && !in.AcknowledgeWarnings {
		return spec, fault.New(fault.Invalid, "warnings_not_acknowledged",
			"Review the warnings, then publish again acknowledging them.").With("warnings", report.Warnings)
	}

	return spec, nil
}

/*
via names how a publisher acted, session when unsaid.
*/
func via(kind string) *string {
	if strings.TrimSpace(kind) == "" {
		kind = "session"
	}

	return &kind
}

/*
changed tells the live cache the workspace's live workflows changed.
*/
func (w *Workflows) changed(ctx context.Context, workspace ids.UUID) {
	if w.Live != nil {
		w.Live.WorkflowsChanged(ctx, workspace)
	}
}
