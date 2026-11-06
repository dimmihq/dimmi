/*
|--------------------------------------------------------------------------
| Workspace Logo Controller
|--------------------------------------------------------------------------
|
| PUT /workspace/logo replaces the workspace's logo from a multipart
| upload, and DELETE /workspace/logo takes it down.
|
*/

package v1

import (
	"context"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
WorkspaceLogoController serves the logo of the workspace in play.
*/
type WorkspaceLogoController struct {
	UpdateLogo *workspaces.UpdateLogo
	DeleteLogo *workspaces.DeleteLogo
}

/*
Update stores the logo and answers with the workspace as it now is.
*/
func (c *WorkspaceLogoController) Update(
	ctx context.Context, in *requests.UpdateWorkspaceLogoRequest,
) (*WorkspaceOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()

	uploads := in.RawBody.File["logo"]
	if len(uploads) == 0 {
		return nil, exceptions.ErrInvalidAvatar
	}

	file, err := uploads[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded logo: %w", err)
	}

	defer func() { _ = file.Close() }()

	updated, err := c.UpdateLogo.Handle(ctx, membership(ctx), file)
	if err != nil {
		return nil, err
	}

	return &WorkspaceOutput{Body: fhttp.NewResource(resources.NewWorkspace(updated))}, nil
}

/*
Destroy takes the logo down.
*/
func (c *WorkspaceLogoController) Destroy(ctx context.Context, _ *requests.NoInput) (*EmptyOutput, error) {
	if err := c.DeleteLogo.Handle(ctx, membership(ctx)); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}
