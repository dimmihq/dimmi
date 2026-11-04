/*
|--------------------------------------------------------------------------
| Invitation Delivery Controller
|--------------------------------------------------------------------------
|
| POST /invitations/{invitation}/deliveries sends an invitation again
| with a fresh link. Nothing is stored about the sending itself beyond
| the invitation's count and time of the last one, so the answer is
| worked out from the invitation.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
InvitationDeliveryOutput is one more sending of an invitation.
*/
type InvitationDeliveryOutput struct {
	Body fhttp.Resource[resources.InvitationDelivery]
}

/*
InvitationDeliveryController sends invitations of the workspace in
play again.
*/
type InvitationDeliveryController struct {
	Resend *invitations.ResendInvitation
	IDs    *ids.Registry
}

/*
Store sends the invitation again and answers with the sending.
*/
func (c *InvitationDeliveryController) Store(
	ctx context.Context, in *requests.InvitationRequest,
) (*InvitationDeliveryOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInvitation), in.Invitation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	sent, err := c.Resend.Handle(ctx, invitations.ChangeInvitationInput{
		Membership: membership(ctx), Caller: caller(ctx).User, InvitationID: id,
	})
	if err != nil {
		return nil, err
	}

	return &InvitationDeliveryOutput{Body: fhttp.NewResource(resources.NewInvitationDelivery(sent))}, nil
}
