/*
|--------------------------------------------------------------------------
| Workspace Controller
|--------------------------------------------------------------------------
|
| PATCH /workspaces/{workspace} with trashed: false takes a workspace out
| of the trash. It names the workspace in the address rather than in the
| Dimmi-Workspace header, since a workspace in the trash is one nobody
| can work in, and only its owners find it at all.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkspaceController serves the caller's workspaces by id.
*/
type WorkspaceController struct {
	Restore *workspaces.RestoreWorkspace
	IDs     *ids.Registry
}

/*
Update restores the workspace and answers with it as it now is.
*/
func (c *WorkspaceController) Update(
	ctx context.Context, in *requests.UpdateWorkspaceRequest,
) (*WorkspaceOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixWorkspace), in.Workspace)
	if err != nil {
		return nil, exceptions.ErrWorkspaceNotFound
	}

	restored, err := c.Restore.Handle(ctx, workspaces.RestoreWorkspaceInput{Principal: caller(ctx), WorkspaceID: id})
	if err != nil {
		return nil, err
	}

	return &WorkspaceOutput{Body: fhttp.NewResource(resources.NewWorkspace(restored))}, nil
}
