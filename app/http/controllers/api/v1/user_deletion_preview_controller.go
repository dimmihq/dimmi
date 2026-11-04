/*
|--------------------------------------------------------------------------
| User Deletion Preview Controller
|--------------------------------------------------------------------------
|
| GET /user/deletion-preview says what deleting the account would do, so
| the person sees what stands in the way before they are asked to confirm.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
DeletionPreviewOutput answers with what deleting the account would do.
*/
type DeletionPreviewOutput struct {
	Body fhttp.Resource[resources.DeletionPreview]
}

/*
UserDeletionPreviewController serves the deletion preview.
*/
type UserDeletionPreviewController struct {
	Preview *members.PreviewAccountDeletion
}

/*
Show answers with the preview.
*/
func (c *UserDeletionPreviewController) Show(
	ctx context.Context, _ *requests.NoInput,
) (*DeletionPreviewOutput, error) {
	preview, err := c.Preview.Handle(ctx, caller(ctx).ID())
	if err != nil {
		return nil, err
	}

	return &DeletionPreviewOutput{Body: fhttp.NewResource(resources.NewDeletionPreview(preview))}, nil
}
