/*
|--------------------------------------------------------------------------
| Invitation Preview Controller
|--------------------------------------------------------------------------
|
| GET /invitations/preview shows whoever holds an invitation link what
| it invites them to, signed in or not. The link's secret comes in the
| Dimmi-Invitation-Token header, never in the address, and a link of no
| invitation is simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
InvitationPreviewOutput is an invitation as the holder of its link
sees it.
*/
type InvitationPreviewOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         fhttp.Resource[resources.InvitationPreview]
}

/*
InvitationPreviewController shows invitations to the holders of their
links.
*/
type InvitationPreviewController struct {
	Preview *invitations.PreviewInvitation
}

/*
Show answers with the invitation the link belongs to.
*/
func (c *InvitationPreviewController) Show(
	ctx context.Context, in *requests.PreviewInvitationRequest,
) (*InvitationPreviewOutput, error) {
	preview, err := c.Preview.Handle(ctx, in.Token)
	if err != nil {
		return nil, err
	}

	return &InvitationPreviewOutput{
		CacheControl: "no-store", Body: fhttp.NewResource(resources.NewInvitationPreview(preview)),
	}, nil
}
