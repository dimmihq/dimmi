/*
|--------------------------------------------------------------------------
| Team Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on a workspace's teams. A team is named by its
| public id. A name must keep something visible once cleaned. Members
| are added up to a hundred at a time, each named by their public id.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
ShowTeamRequest is the input of GET /teams/{team} and DELETE
/teams/{team}.
*/
type ShowTeamRequest struct {
	Team string `path:"team"`
}

/*
StoreTeamRequest is the input of POST /teams.
*/
type StoreTeamRequest struct {
	Body struct {
		Name string `json:"name" validate:"required,max=80"`
	}
}

/*
Validate refuses a name made only of invisible characters.
*/
func (r *StoreTeamRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	return visibleName(ctx, r.Body.Name)
}

/*
UpdateTeamRequest is the input of PATCH /teams/{team}.
*/
type UpdateTeamRequest struct {
	Team string `path:"team"`
	Body struct {
		Name string `json:"name" validate:"required,max=80"`
	}
}

/*
Validate refuses a name made only of invisible characters.
*/
func (r *UpdateTeamRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	return visibleName(ctx, r.Body.Name)
}

/*
StoreTeamMembersRequest is the input of POST /teams/{team}/members.
*/
type StoreTeamMembersRequest struct {
	Team string `path:"team"`
	Body struct {
		MemberIDs []string `json:"member_ids" validate:"required,min=1,max=100"`
	}
}

/*
DestroyTeamMemberRequest is the input of DELETE
/teams/{team}/members/{member}.
*/
type DestroyTeamMemberRequest struct {
	Team   string `path:"team"`
	Member string `path:"member"`
}

/*
visibleName refuses a name that is empty once cleaned.
*/
func visibleName(ctx context.Context, name string) error {
	if str.Clean(name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}
