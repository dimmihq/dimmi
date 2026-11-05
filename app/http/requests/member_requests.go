/*
|--------------------------------------------------------------------------
| Member Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on a workspace's members. A member is named by
| their public id, or by "me" for the caller's own membership. An update
| carries a role, a status or both; the side fields only make sense with
| the field they belong to, so a reassignment target comes only with a
| deactivation, and keeping ownership only with handing it over. A
| capacity sent as null means no limit.
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
UpdateMemberBody is what an update of a member may change.
*/
type UpdateMemberBody struct {
	AssignedRole       *string `json:"role,omitempty" validate:"omitempty,oneof=owner admin member"`
	Status             *string `json:"status,omitempty" validate:"omitempty,oneof=active deactivated"`
	ReassignToMemberID *string `json:"reassign_to_member_id,omitempty"`
	KeepOwner          *bool   `json:"keep_owner,omitempty"`

	Access *string `json:"conversation_access,omitempty" validate:"omitempty,oneof=all assigned teams all_except_teams"`

	AccessUnassigned *bool     `json:"conversation_access_unassigned,omitempty"`
	AccessTeamIDs    *[]string `json:"access_team_ids,omitempty"`

	Availability      *string         `json:"availability,omitempty" validate:"omitempty,oneof=online away"`
	AwayReassign      *bool           `json:"away_reassign,omitempty"`
	AssignOnReply     *string         `json:"assign_on_reply,omitempty" validate:"omitempty,oneof=inherit always never"`
	Capacity          *int32          `json:"conversation_capacity,omitempty" validate:"omitempty,min=1,max=500"`
	CapacityByChannel *map[string]int `json:"capacity_by_channel,omitempty" validate:"omitempty,dive,min=1,max=500"`
}

/*
UpdateMemberRequest is the input of PATCH /members/{member}.
*/
type UpdateMemberRequest struct {
	Member  string `path:"member"`
	Body    UpdateMemberBody
	RawBody []byte
}

/*
UpdateOwnMemberRequest is the input of PATCH /members/me, which a
token able to write conversations may use to set its own availability.
*/
type UpdateOwnMemberRequest struct {
	Body    UpdateMemberBody
	RawBody []byte
}

/*
Validate refuses a side field without the field it belongs to.
*/
func (r *UpdateOwnMemberRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	return validateUpdate(ctx, r.Body)
}

/*
Validate refuses a side field without the field it belongs to.
*/
func (r *UpdateMemberRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	return validateUpdate(ctx, r.Body)
}

/*
validateUpdate refuses a side field without the field it belongs to,
and a capacity for a channel type that does not exist.
*/
func validateUpdate(ctx context.Context, b UpdateMemberBody) error {
	if b.ReassignToMemberID != nil && (b.Status == nil || *b.Status != "deactivated") {
		return violation(ctx, "reassign_to_member_id", "prohibited", "validation.prohibited")
	}

	if b.KeepOwner != nil && (b.AssignedRole == nil || *b.AssignedRole != "owner") {
		return violation(ctx, "keep_owner", "prohibited", "validation.prohibited")
	}

	if b.CapacityByChannel == nil {
		return nil
	}

	for channel := range *b.CapacityByChannel {
		if err := choice(ctx, "capacity_by_channel", &channel, capacityChannels); err != nil {
			return err
		}
	}

	return nil
}

/*
capacityChannels are the channel types a member's capacity may name.
*/
const capacityChannels = "widget email api"
