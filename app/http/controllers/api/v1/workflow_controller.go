/*
|--------------------------------------------------------------------------
| Workflow Controller
|--------------------------------------------------------------------------
|
| A workspace's workflows: listed in priority order, created from a
| trigger or a whole definition, saved as drafts, published, paused,
| rolled back, archived and reordered, with their version history.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowController serves workflows.
*/
type WorkflowController struct {
	Workflows *workflows.Workflows
	IDs       *ids.Registry
}

/*
WorkflowsOutput is a list of workflows.
*/
type WorkflowsOutput struct {
	Body fhttp.Collection[resources.Workflow]
}

/*
WorkflowOutput is one workflow with its working copy.
*/
type WorkflowOutput struct {
	Body fhttp.Resource[resources.Workflow]
}

/*
WorkflowVersionsOutput is a workflow's versions.
*/
type WorkflowVersionsOutput struct {
	Body fhttp.Collection[resources.WorkflowVersion]
}

/*
PublicationOutput is a publish: the workflow and the version it made.
*/
type PublicationOutput struct {
	Body struct {
		Data struct {
			Workflow resources.Workflow        `json:"workflow"`
			Version  resources.WorkflowVersion `json:"version"`
		} `json:"data"`
	}
}

/*
Index lists the workflows in priority order.
*/
func (c *WorkflowController) Index(ctx context.Context, in *requests.IndexWorkflowsRequest) (*WorkflowsOutput, error) {
	rows, err := c.Workflows.List(ctx, membership(ctx), in.Archived)
	return workflowsOutput(rows, err)
}

/*
Store creates a draft workflow.
*/
func (c *WorkflowController) Store(ctx context.Context, in *requests.StoreWorkflowRequest) (*WorkflowOutput, error) {
	w, err := c.Workflows.Create(ctx, workflows.CreateInput{
		Membership: membership(ctx), Name: in.Body.Name, Description: in.Body.Description, Trigger: in.Body.Trigger,
		Spec: in.Body.Spec,
	})

	return workflowOutput(w, err)
}

/*
Show answers one workflow with its working copy.
*/
func (c *WorkflowController) Show(ctx context.Context, in *requests.WorkflowRequest) (*WorkflowOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	w, err := c.Workflows.Show(ctx, membership(ctx), id)

	return workflowOutput(w, err)
}

/*
Update saves the working copy, changes the status, or restores a
version.
*/
func (c *WorkflowController) Update(ctx context.Context, in *requests.UpdateWorkflowRequest) (*WorkflowOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	b, m := in.Body, membership(ctx)
	switch {
	case b.Status != nil:
		w, err := c.Workflows.SetStatus(ctx, m, id, *b.Status)
		return workflowOutput(w, err)
	case b.RestoreVersionID != nil:
		return c.restore(ctx, id, in)
	}

	w, err := c.Workflows.Update(ctx, workflows.UpdateInput{
		Membership: m, ID: id, LockVersion: b.LockVersion, Name: b.Name, Description: b.Description, Spec: b.Spec,
		TestOnTest: b.TestOnTestConversations,
	})

	return workflowOutput(w, err)
}

/*
restore copies a version into the working copy.
*/
func (c *WorkflowController) restore(ctx context.Context, id ids.UUID, in *requests.UpdateWorkflowRequest,
) (*WorkflowOutput, error) {
	version, err := c.decode(models.PrefixWorkflowVersion, *in.Body.RestoreVersionID)
	if err != nil {
		return nil, workflows.ErrVersionNotFound
	}

	p, err := c.Workflows.Restore(ctx, workflows.RestoreInput{
		Membership: membership(ctx), ID: id, VersionID: version, Publish: in.Body.Publish,
		AcknowledgeWarnings: in.Body.AcknowledgeWarnings,
	})

	return workflowOutput(p.Workflow, err)
}

/*
Reorder puts the workflows in a new order.
*/
func (c *WorkflowController) Reorder(ctx context.Context, in *requests.ReorderWorkflowsRequest,
) (*WorkflowsOutput, error) {
	order := make([]ids.UUID, 0, len(in.Body.Order))
	for _, public := range in.Body.Order {
		id, err := c.decode(models.PrefixWorkflow, public)
		if err != nil {
			return nil, workflows.ErrOrderStale
		}

		order = append(order, id)
	}

	rows, err := c.Workflows.Reorder(ctx, membership(ctx), order)

	return workflowsOutput(rows, err)
}

/*
Publish makes the working copy live as a new version.
*/
func (c *WorkflowController) Publish(ctx context.Context, in *requests.PublishWorkflowRequest,
) (*PublicationOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	body := requests.PublishWorkflowBody{}
	if in.Body != nil {
		body = *in.Body
	}

	p, err := c.Workflows.Publish(ctx, workflows.PublishInput{
		Membership: membership(ctx), ID: id, Note: body.Note, AcknowledgeWarnings: body.AcknowledgeWarnings,
		DraftHash: body.DraftHash,
	})
	if err != nil {
		return nil, err
	}

	out := &PublicationOutput{}
	out.Body.Data.Workflow = resources.NewWorkflow(p.Workflow)
	out.Body.Data.Version = resources.NewWorkflowVersion(p.Version)

	return out, nil
}

/*
Versions lists a workflow's versions, newest first.
*/
func (c *WorkflowController) Versions(ctx context.Context, in *requests.WorkflowRequest,
) (*WorkflowVersionsOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	rows, err := c.Workflows.Versions(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.WorkflowVersion]{Items: resources.NewWorkflowVersions(rows), Limit: 100}

	return &WorkflowVersionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Destroy archives a workflow.
*/
func (c *WorkflowController) Destroy(ctx context.Context, in *requests.WorkflowRequest) (*struct{}, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	_, err = c.Workflows.Archive(ctx, membership(ctx), id)

	return &struct{}{}, err
}

/*
Unarchive brings an archived workflow back.
*/
func (c *WorkflowController) Unarchive(ctx context.Context, in *requests.WorkflowRequest) (*WorkflowOutput, error) {
	id, err := c.decode(models.PrefixWorkflow, in.Workflow)
	if err != nil {
		return nil, err
	}

	w, err := c.Workflows.Unarchive(ctx, membership(ctx), id)

	return workflowOutput(w, err)
}

func workflowOutput(w models.Workflow, err error) (*WorkflowOutput, error) {
	if err != nil {
		return nil, err
	}

	return &WorkflowOutput{Body: fhttp.NewResource(resources.NewWorkflow(w))}, nil
}

func workflowsOutput(rows []models.Workflow, err error) (*WorkflowsOutput, error) {
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.Workflow]{Items: resources.NewWorkflows(rows), Limit: 500}

	return &WorkflowsOutput{Body: fhttp.NewCollection(page)}, nil
}

func (c *WorkflowController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
