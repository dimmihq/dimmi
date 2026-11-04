/*
|--------------------------------------------------------------------------
| Me Controller
|--------------------------------------------------------------------------
|
| GET /me answers any valid credential with who it is and what it may do,
| in the workspace a browser or app names, or a token's own.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MeController serves the caller's own picture.
*/
type MeController struct {
	LoadMe *auth.LoadMe
	IDs    *ids.Registry
	Server Server
}

/*
Show answers with the caller's picture.
*/
func (c *MeController) Show(ctx context.Context, in *requests.WorkspaceRequest) (*MeOutput, error) {
	p := caller(ctx)

	workspaceID := ids.Nil()
	if in.Workspace != "" {
		var err error

		workspaceID, err = c.IDs.Decode(string(models.PrefixWorkspace), in.Workspace)
		if err != nil {
			return nil, exceptions.ErrWorkspaceNotFound
		}
	}

	me, err := c.LoadMe.Handle(ctx, auth.LoadMeInput{Principal: p, WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}

	return &MeOutput{Body: fhttp.NewResource(resources.NewMe(ctx, me, c.Server.instance()))}, nil
}
