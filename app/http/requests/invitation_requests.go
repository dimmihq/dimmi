/*
|--------------------------------------------------------------------------
| Invitation Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that invite people into a workspace and that
| answer an invitation. Invitations come in batches whose addresses are
| judged one by one, so a bad address is an answer for that address, not
| a refusal of the batch; a role or a batch size that cannot work is.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
InvitationInput is one address of a batch, the role it is offered, the
custom role it starts with, if any, and the teams it joins on accepting.
*/
type InvitationInput struct {
	Email        string   `json:"email"`
	Role         string   `json:"role"                     validate:"required,oneof=admin member"`
	CustomRoleID *string  `json:"custom_role_id,omitempty"`
	TeamIDs      []string `json:"team_ids,omitempty"       validate:"max=50"`
}

/*
StoreInvitationsRequest is the input of POST /invitations.
*/
type StoreInvitationsRequest struct {
	Body struct {
		Invitations []InvitationInput `json:"invitations" validate:"required,min=1,max=50,dive"`
	}
}

/*
InvitationRequest is the input of the endpoints on one invitation.
*/
type InvitationRequest struct {
	Invitation string `path:"invitation"`
}

/*
PreviewInvitationRequest is the input of GET /invitations/preview. The
link's secret rides in a header, never in the address, so it stays out
of logs.
*/
type PreviewInvitationRequest struct {
	Token string `header:"Dimmi-Invitation-Token"`
}

/*
StoreMembershipRequest is the input of POST /user/memberships, which
joins a workspace by accepting an invitation. Joining by any other
means, such as naming a workspace, is refused.
*/
type StoreMembershipRequest struct {
	Body struct {
		InvitationToken string `json:"invitation_token,omitempty" validate:"required,max=200"`
		WorkspaceID     any    `json:"workspace_id,omitempty"`
	}
}

/*
Validate refuses a workspace id.
*/
func (r *StoreMembershipRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.WorkspaceID != nil {
		return violation(ctx, "workspace_id", "prohibited", "validation.prohibited")
	}

	return nil
}
