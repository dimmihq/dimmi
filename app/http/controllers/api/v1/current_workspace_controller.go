/*
|--------------------------------------------------------------------------
| Current Workspace Controller
|--------------------------------------------------------------------------
|
| GET /workspace shows the workspace a request works in to any of its
| members, and PATCH /workspace changes its name, address, language and
| time zone. The workspace is the one the request named, already
| resolved and checked before these run.
|
*/

package v1

import (
	"context"

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
		Membership: membership(ctx), Name: in.Body.Name, Slug: in.Body.Slug, Timezone: in.Body.Timezone,
		Locale: in.Body.Locale, TestMode: in.Body.TestMode,
	})
	if err != nil {
		return nil, err
	}

	return &WorkspaceOutput{Body: fhttp.NewResource(resources.NewWorkspace(updated))}, nil
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
