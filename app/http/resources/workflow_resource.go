/*
|--------------------------------------------------------------------------
| Workflow Resources
|--------------------------------------------------------------------------
|
| How a workflow reads over the API: where it stands, whether its working
| copy differs from what runs, and, for one workflow, the working copy
| itself with everything that would stop it from going live. Versions
| carry the frozen definition they hold.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
Workflow is a workflow. Spec and Report are only set for one workflow:
the working copy and what checking it found.
*/
type Workflow struct {
	ID                      string               `json:"id"`
	Object                  string               `json:"object"`
	Name                    string               `json:"name"`
	Description             *string              `json:"description"`
	Trigger                 string               `json:"trigger"`
	Status                  string               `json:"status"`
	Kind                    string               `json:"kind"`
	Position                int32                `json:"position"`
	LockVersion             int32                `json:"lock_version"`
	DraftHash               string               `json:"draft_hash"`
	HasUnpublishedChanges   bool                 `json:"has_unpublished_changes"`
	LiveVersionID           *string              `json:"live_version_id"`
	TestOnTestConversations bool                 `json:"test_on_test_conversations"`
	Archived                bool                 `json:"archived"`
	Spec                    json.RawMessage      `json:"spec,omitempty"`
	Report                  *workflowspec.Report `json:"report,omitempty"`
	PublishedAt             *time.Time           `json:"published_at"`
	PausedAt                *time.Time           `json:"paused_at"`
	CreatedAt               time.Time            `json:"created_at"`
	UpdatedAt               time.Time            `json:"updated_at"`
}

/*
NewWorkflow shapes one workflow with its working copy and its report.
*/
func NewWorkflow(w models.Workflow) Workflow {
	out := workflowSummary(w)
	out.Spec = w.Draft

	if spec, err := workflowspec.Parse(w.Draft); err == nil {
		report := workflowspec.Check(spec)
		out.Report = &report
	}

	return out
}

/*
NewWorkflows shapes a list of workflows without their definitions.
*/
func NewWorkflows(rows []models.Workflow) []Workflow {
	out := make([]Workflow, len(rows))
	for i, w := range rows {
		out[i] = workflowSummary(w)
	}

	return out
}

/*
workflowSummary shapes a workflow without its definition.
*/
func workflowSummary(w models.Workflow) Workflow {
	out := Workflow{
		ID: models.PublicID(models.PrefixWorkflow, w.ID), Object: "workflow", Name: w.Name,
		Description: w.Description, Trigger: w.Trigger, Status: w.Status, Kind: w.Kind, Position: w.Position,
		LockVersion: w.LockVersion, DraftHash: w.DraftHash, TestOnTestConversations: w.TestOnTestConversations,
		Archived: w.DeletedAt != nil, PublishedAt: w.PublishedAt, PausedAt: w.PausedAt, CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}

	out.HasUnpublishedChanges = w.LiveHash == nil || *w.LiveHash != w.DraftHash
	if w.LiveVersionID != nil {
		id := models.PublicID(models.PrefixWorkflowVersion, *w.LiveVersionID)
		out.LiveVersionID = &id
	}

	return out
}

/*
WorkflowVersion is a published version of a workflow.
*/
type WorkflowVersion struct {
	ID                    string          `json:"id"`
	Object                string          `json:"object"`
	WorkflowID            string          `json:"workflow_id"`
	Number                int32           `json:"number"`
	Kind                  string          `json:"kind"`
	Note                  *string         `json:"note"`
	PublishedVia          *string         `json:"published_via"`
	PublishedBy           *string         `json:"published_by_member_id"`
	RestoredFromVersionID *string         `json:"restored_from_version_id"`
	Spec                  json.RawMessage `json:"spec"`
	CreatedAt             time.Time       `json:"created_at"`
}

/*
NewWorkflowVersion shapes one version.
*/
func NewWorkflowVersion(v models.WorkflowVersion) WorkflowVersion {
	out := WorkflowVersion{
		ID: models.PublicID(models.PrefixWorkflowVersion, v.ID), Object: "workflow_version",
		WorkflowID: models.PublicID(models.PrefixWorkflow, v.WorkflowID), Number: v.Number, Kind: v.Kind,
		Note: v.Note, PublishedVia: v.PublishedVia, Spec: v.Spec, CreatedAt: v.CreatedAt,
	}

	if v.PublishedByMemberID != nil {
		id := models.PublicID(models.PrefixMember, *v.PublishedByMemberID)
		out.PublishedBy = &id
	}

	if v.RestoredFromVersionID != nil {
		id := models.PublicID(models.PrefixWorkflowVersion, *v.RestoredFromVersionID)
		out.RestoredFromVersionID = &id
	}

	return out
}

/*
NewWorkflowVersions shapes a workflow's versions.
*/
func NewWorkflowVersions(rows []models.WorkflowVersion) []WorkflowVersion {
	out := make([]WorkflowVersion, len(rows))
	for i, v := range rows {
		out[i] = NewWorkflowVersion(v)
	}

	return out
}
