/*
|--------------------------------------------------------------------------
| Current Workspace Controller
|--------------------------------------------------------------------------
|
| GET /workspace shows the workspace a request works in to any of its
| members, PATCH /workspace changes its name, address, language, time
| zone and whether it requires a second factor, and DELETE /workspace
| moves it to the trash. The workspace is the one the request named,
| already resolved and checked before these run.
|
*/

package v1

import (
	"context"
	"encoding/json"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
WorkspaceOutput is one workspace.
*/
type WorkspaceOutput struct {
	Body fhttp.Resource[resources.Workspace]
}

/*
CurrentWorkspaceController serves the workspace in play.
*/
type CurrentWorkspaceController struct {
	UpdateWorkspace *workspaces.UpdateWorkspace
	DeleteWorkspace *workspaces.DeleteWorkspace
}

/*
Show answers with the workspace.
*/
func (c *CurrentWorkspaceController) Show(ctx context.Context, _ *requests.NoInput) (*WorkspaceOutput, error) {
	return &WorkspaceOutput{Body: fhttp.NewResource(resources.NewWorkspace(membership(ctx).Workspace))}, nil
}

/*
Update changes the workspace and answers with it as it now is.
*/
func (c *CurrentWorkspaceController) Update(
	ctx context.Context, in *requests.UpdateCurrentWorkspaceRequest,
) (*WorkspaceOutput, error) {
	updated, err := c.UpdateWorkspace.Handle(ctx, workspaces.UpdateWorkspaceInput{
		Principal: caller(ctx), Membership: membership(ctx), Name: in.Body.Name, Slug: in.Body.Slug,
		Timezone: in.Body.Timezone, Locale: in.Body.Locale, TestMode: in.Body.TestMode,
		RequireTwoFactor: in.Body.RequireTwoFactor, CustomerFacingName: in.Body.CustomerFacingName,
		ClearCustomerFacingName: requests.Nulls(in.RawBody)["customer_facing_name"],
		ShowViewers:             in.Body.ShowViewers,
		UndoSendSeconds:         in.Body.UndoSendSeconds,
		SeenReceiptName:         in.Body.SeenReceiptName,
		GIFPicker:               in.Body.GIFPicker,
		TeamMentions:            in.Body.TeamMentions,
		LinkSafety:              linkSafetyChange(in.Body.LinkSafety),
		Retention:               retentionChange(in.Body.Retention, in.RawBody),
	})
	if err != nil {
		return nil, err
	}

	return &WorkspaceOutput{Body: fhttp.NewResource(resources.NewWorkspace(updated))}, nil
}

/*
Destroy moves the workspace to the trash and answers with it there.
*/
func (c *CurrentWorkspaceController) Destroy(
	ctx context.Context, in *requests.DestroyCurrentWorkspaceRequest,
) (*WorkspaceOutput, error) {
	trashed, err := c.DeleteWorkspace.Handle(ctx, workspaces.DeleteWorkspaceInput{
		Principal: caller(ctx), Membership: membership(ctx), ConfirmSlug: in.Body.ConfirmSlug,
	})
	if err != nil {
		return nil, err
	}

	return &WorkspaceOutput{Body: fhttp.NewResource(resources.NewWorkspace(trashed))}, nil
}

/*
membership returns the caller's membership of the workspace in play.
Only routes behind the workspace middleware call it, and that
middleware stores one or refuses the request.
*/
func membership(ctx context.Context) authz.Membership {
	m, _ := authz.CurrentMembership(ctx)

	return m
}

/*
linkSafetyChange reads the link checks sent, nil when none were.
*/
func linkSafetyChange(body *requests.LinkSafetyBody) *workspaces.LinkSafetyChange {
	if body == nil {
		return nil
	}

	return &workspaces.LinkSafetyChange{
		Enabled: body.Enabled, AllowDomains: body.AllowDomains, BlockDomains: body.BlockDomains,
		StripInboundEmailLinks: body.StripInboundEmailLinks,
	}
}

/*
retentionChange reads the retention periods sent, nil when none were,
telling closed_conversation_days sent as null from one left out.
*/
func retentionChange(body *requests.RetentionBody, raw []byte) *workspaces.RetentionChange {
	if body == nil {
		return nil
	}

	sent := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &sent)

	return &workspaces.RetentionChange{
		ClosedConversationDays:      body.ClosedConversationDays,
		ClearClosedConversationDays: requests.Nulls(sent["retention"])["closed_conversation_days"],
		SpamDays:                    body.SpamDays, TrashDays: body.TrashDays, RawEmailDays: body.RawEmailDays,
		ExportDays: body.ExportDays,
	}
}
