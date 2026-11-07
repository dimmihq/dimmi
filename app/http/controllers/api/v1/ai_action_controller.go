/*
|--------------------------------------------------------------------------
| Agent Action Controller
|--------------------------------------------------------------------------
|
| The workspace's actions: listed, drafted, edited, deleted, published
| as versions and tested. What a field may hold is the actions' to
| decide; the controller only maps public ids and tells a body field
| sent as null from one left out.
|
*/

package v1

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIActionController serves the actions.
*/
type AIActionController struct {
	Definitions *agentactions.Definitions
	IDs         *ids.Registry
}

/*
AIActionsOutput is the action list.
*/
type AIActionsOutput struct {
	Body fhttp.Collection[resources.AIAction]
}

/*
AIActionOutput is one action.
*/
type AIActionOutput struct {
	Body fhttp.Resource[resources.AIAction]
}

/*
Index lists the actions.
*/
func (c *AIActionController) Index(ctx context.Context, in *requests.AIActionsRequest) (*AIActionsOutput, error) {
	f := agentactions.ListFilter{}
	if in.Status != "" {
		f.Status = &in.Status
	}
	if in.Connection != "" {
		id, err := c.decode(models.PrefixAiActionConnection, in.Connection)
		if err != nil {
			return nil, err
		}
		f.ConnectionID = &id
	}
	rows, err := c.Definitions.List(ctx, membership(ctx), f)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.AIAction]{Items: resources.NewAIActions(rows), Limit: 500}
	return &AIActionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store drafts an action.
*/
func (c *AIActionController) Store(ctx context.Context, in *requests.AIStoreActionRequest) (*AIActionOutput, error) {
	input, err := c.input(ctx, in.Body, in.RawBody)
	if err != nil {
		return nil, err
	}
	view, err := c.Definitions.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return &AIActionOutput{Body: fhttp.NewResource(resources.NewAIAction(view))}, nil
}

/*
Show answers one action.
*/
func (c *AIActionController) Show(ctx context.Context, in *requests.AIActionRequest) (*AIActionOutput, error) {
	id, err := c.decode(models.PrefixAiAction, in.Action)
	if err != nil {
		return nil, err
	}
	view, err := c.Definitions.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return &AIActionOutput{Body: fhttp.NewResource(resources.NewAIAction(view))}, nil
}

/*
Update edits an action or switches it on or off.
*/
func (c *AIActionController) Update(ctx context.Context, in *requests.AIUpdateActionRequest) (*AIActionOutput, error) {
	id, err := c.decode(models.PrefixAiAction, in.Action)
	if err != nil {
		return nil, err
	}
	input, err := c.input(ctx, in.Body, in.RawBody)
	if err != nil {
		return nil, err
	}
	view, err := c.Definitions.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}
	return &AIActionOutput{Body: fhttp.NewResource(resources.NewAIAction(view))}, nil
}

/*
Destroy deletes an action.
*/
func (c *AIActionController) Destroy(ctx context.Context, in *requests.AIActionRequest) (*EmptyOutput, error) {
	id, err := c.decode(models.PrefixAiAction, in.Action)
	if err != nil {
		return nil, err
	}
	return &EmptyOutput{}, c.Definitions.Delete(ctx, membership(ctx), id)
}

/*
Publish publishes the working copy as a new version.
*/
func (c *AIActionController) Publish(ctx context.Context, in *requests.AIPublishActionRequest,
) (*AIActionVersionOutput, error) {
	id, err := c.decode(models.PrefixAiAction, in.Action)
	if err != nil {
		return nil, err
	}
	v, err := c.Definitions.Publish(ctx, membership(ctx), id, in.Body.Note)
	if err != nil {
		return nil, err
	}
	names := c.Definitions.PublisherNames(ctx, membership(ctx), []models.AiActionVersion{v})
	return &AIActionVersionOutput{Body: fhttp.NewResource(resources.NewAIActionVersion(v, names))}, nil
}

/*
Versions lists an action's versions.
*/
func (c *AIActionController) Versions(ctx context.Context, in *requests.AIActionRequest,
) (*AIActionVersionsOutput, error) {
	id, err := c.decode(models.PrefixAiAction, in.Action)
	if err != nil {
		return nil, err
	}
	rows, err := c.Definitions.Versions(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	names := c.Definitions.PublisherNames(ctx, membership(ctx), rows)
	items := make([]resources.AIActionVersion, len(rows))
	for i, v := range rows {
		items[i] = resources.NewAIActionVersion(v, names)
	}
	page := pagination.Page[resources.AIActionVersion]{Items: items, Limit: 100}
	return &AIActionVersionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Test runs a test of the working copy.
*/
func (c *AIActionController) Test(ctx context.Context, in *requests.AITestActionRequest) (*AIActionTestOutput, error) {
	id, err := c.decode(models.PrefixAiAction, in.Action)
	if err != nil {
		return nil, err
	}
	b := in.Body
	input := agentactions.TestInput{
		Membership: membership(ctx), ActionID: id, Input: b.Input, Mode: b.Mode,
		ConfirmSideEffects: b.ConfirmSideEffects,
	}
	if input.Mode == "" {
		input.Mode = "dry_run"
	}
	if b.Contact != nil {
		input.Contact = agentactions.TestContact{
			ExternalID: b.Contact.ExternalID, Email: b.Contact.Email, Attributes: b.Contact.CustomAttributes,
		}
		if input.Contact.Attributes == nil {
			input.Contact.Attributes = b.Contact.Attributes
		}
	}
	res, err := c.Definitions.Test(ctx, input)
	if err != nil {
		return nil, err
	}
	return &AIActionTestOutput{Body: fhttp.NewResource(resources.NewAIActionTest(res))}, nil
}

/*
input maps a request body to the action's input.
*/
func (c *AIActionController) input(ctx context.Context, b requests.AIActionBody, raw []byte,
) (agentactions.DefinitionInput, error) {
	in := agentactions.DefinitionInput{
		Membership: membership(ctx), Name: b.Name, Title: b.Title, Description: b.Description,
		SideEffect: b.SideEffect, Confirmation: b.Confirmation, Approval: b.Approval,
		ApprovalTimeoutMinutes: b.ApprovalTimeoutMinutes, Method: b.Method, PathTemplate: b.PathTemplate,
		Query: b.Query, Headers: b.Headers, BodyEncoding: b.BodyEncoding, MCPToolName: b.MCPToolName,
		BoundArguments: b.BoundArguments, ResponseFields: b.ResponseFields, ErrorMessagePath: b.ErrorMessagePath,
		ResponseMaxBytes: b.ResponseMaxBytes, TimeoutMS: b.TimeoutMS, Channels: b.Channels, Status: b.Status,
		Source: "api",
	}
	sent := requests.Sent(raw)
	if sent["body_template"] {
		body := json.RawMessage("null")
		if len(b.BodyTemplate) > 0 {
			body = b.BodyTemplate
		}
		in.BodyTemplate = &body
	}
	if sent["input_schema"] && len(b.InputSchema) > 0 {
		schema := b.InputSchema
		in.InputSchema = &schema
	}
	if b.ConnectionID != nil {
		id, err := c.decode(models.PrefixAiActionConnection, *b.ConnectionID)
		if err != nil {
			return in, invalidConnection()
		}
		in.ConnectionID = &id
	}
	if b.RestoreVersionID != nil {
		id, err := c.decode(models.PrefixAiActionVersion, *b.RestoreVersionID)
		if err != nil {
			return in, err
		}
		in.RestoreVersionID = &id
	}
	return in, nil
}

/*
decode reads a public id of the kind; an unknown one is not found.
*/
func (c *AIActionController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}

/*
invalidConnection is the error of a connection id that is not one.
*/
func invalidConnection() error {
	return fault.New(fault.Invalid, "connection_invalid", "the connection does not exist").OnField("connection_id")
}
