/*
|--------------------------------------------------------------------------
| Role Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on a workspace's custom roles. A role is named
| by its public id. Its base role is chosen when it is made and never
| sent again. Members change in bulk up to a hundred at a time, with the
| same fields a single change takes.
|
*/

package requests

import (
	"context"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
ShowRoleRequest is the input of GET /roles/{role} and DELETE
/roles/{role}.
*/
type ShowRoleRequest struct {
	Role string `path:"role"`
}

/*
StoreRoleRequest is the input of POST /roles.
*/
type StoreRoleRequest struct {
	Body struct {
		Name        string   `json:"name"                  validate:"required,max=50"`
		Description *string  `json:"description,omitempty" validate:"omitempty,max=200"`
		BaseRole    string   `json:"base_role"             validate:"required,oneof=admin member"`
		Permissions []string `json:"permissions"           validate:"required,max=200,dive,max=80"`
	}
}

/*
Validate refuses a name made only of invisible characters.
*/
func (r *StoreRoleRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	return visibleName(ctx, r.Body.Name)
}

/*
UpdateRoleRequest is the input of PATCH /roles/{role}.
*/
type UpdateRoleRequest struct {
	Role string `path:"role"`
	Body struct {
		Name        *string   `json:"name,omitempty"        validate:"omitempty,max=50"`
		Description *string   `json:"description,omitempty" validate:"omitempty,max=200"`
		BaseRole    *string   `json:"base_role,omitempty"`
		Permissions *[]string `json:"permissions,omitempty" validate:"omitempty,max=200,dive,max=80"`
	}
}

/*
Validate refuses a name made only of invisible characters, and any try
to change the base role.
*/
func (r *UpdateRoleRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.BaseRole != nil {
		return violation(ctx, "base_role", "prohibited", "validation.prohibited")
	}

	if r.Body.Name != nil {
		return visibleName(ctx, *r.Body.Name)
	}

	return nil
}

/*
UpdateMembersRequest is the input of PATCH /members: the members and the
change each of them gets. A custom role sent as null takes it away, so
the raw body says whether it was sent.
*/
type UpdateMembersRequest struct {
	Body struct {
		IDs     []string `json:"ids" validate:"required,min=1,max=100"`
		Changes struct {
			AssignedRole *string `json:"role,omitempty"           validate:"omitempty,oneof=admin member"`
			CustomRoleID *string `json:"custom_role_id,omitempty"`
		} `json:"changes"`
	}
	RawBody []byte
}
