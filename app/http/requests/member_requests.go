/*
|--------------------------------------------------------------------------
| Member Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on a workspace's members. A member is named by
| their public id, or by "me" for the caller's own membership. An update
| carries a role, a status or both; the side fields only make sense with
| the field they belong to, so a reassignment target comes only with a
| deactivation, and keeping ownership only with handing it over.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
CurrentMember is how a request names the caller's own membership.
*/
const CurrentMember = "me"

/*
ShowMemberRequest is the input of GET /members/{member}.
*/
type ShowMemberRequest struct {
	Member string `path:"member"`
}

/*
UpdateMemberRequest is the input of PATCH /members/{member}.
*/
type UpdateMemberRequest struct {
	Member string `path:"member"`
	Body   struct {
		AssignedRole       *string `json:"role,omitempty" validate:"omitempty,oneof=owner admin member"`
		Status             *string `json:"status,omitempty" validate:"omitempty,oneof=active deactivated"`
		ReassignToMemberID *string `json:"reassign_to_member_id,omitempty"`
		KeepOwner          *bool   `json:"keep_owner,omitempty"`
	}
}

/*
Validate refuses a side field without the field it belongs to.
*/
func (r *UpdateMemberRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	b := r.Body

	if b.ReassignToMemberID != nil && (b.Status == nil || *b.Status != "deactivated") {
		return violation(ctx, "reassign_to_member_id", "prohibited", "validation.prohibited")
	}

	if b.KeepOwner != nil && (b.AssignedRole == nil || *b.AssignedRole != "owner") {
		return violation(ctx, "keep_owner", "prohibited", "validation.prohibited")
	}

	return nil
}
